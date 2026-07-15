package billing

import (
	"net/http"
	"time"

	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RequireActiveSubscription bloquea con 402 el resto de la API una vez que
// el doctor del consultorio (el mismo para su cuenta y la de su personal,
// ver auth.GenerateStaffToken) ya no tiene ni prueba gratis vigente ni una
// suscripción activa. Debe montarse DESPUÉS de auth.AuthorizeJWT() (usa
// "doctorID" del contexto) y NUNCA sobre el propio grupo de rutas de
// facturación (si no, un doctor con la prueba vencida no podría ni pagar).
func RequireActiveSubscription(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.Select("subscription_status, trial_ends_at").First(&doctor, doctorID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "No se pudo verificar tu suscripción"})
			return
		}

		if doctor.SubscriptionStatus == "active" {
			c.Next()
			return
		}

		if doctor.SubscriptionStatus == "trialing" && doctor.TrialEndsAt != nil && time.Now().UTC().Before(*doctor.TrialEndsAt) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
			"error":   "subscription_required",
			"message": "Tu periodo de prueba terminó. Activa tu suscripción para seguir usando ProPatient.",
		})
	}
}
