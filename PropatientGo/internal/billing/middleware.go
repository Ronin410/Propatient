package billing

import (
	"net/http"
	"time"

	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PastDuePaymentGraceDuration: cuánto tiempo sigue con acceso una
// suscripción individual o de clínica después de que su PRIMER cobro
// falla (estado "past_due"), antes de que RequireActiveSubscription
// empiece a bloquear. Sin esto, un solo intento fallido (tarjeta vencida,
// CVV incorrecto, banco la rechazó una vez) corta el acceso de inmediato,
// aunque Stripe mismo siga reintentando el cobro en automático durante
// más o menos este mismo periodo — la idea es darle tiempo real al dueño
// de actualizar su tarjeta antes de perder acceso, no solo el margen que
// tarda Stripe en reintentar.
const PastDuePaymentGraceDuration = 7 * 24 * time.Hour

// withinPastDueGrace decide si una suscripción en "past_due" todavía debe
// tener acceso. pastDueSince nil (nunca se registró cuándo empezó, p. ej.
// datos de antes de este campo existir) se trata como fuera de gracia —
// más seguro fallar hacia bloquear que hacia dejar pasar indefinidamente.
func withinPastDueGrace(pastDueSince *time.Time) bool {
	return pastDueSince != nil && time.Now().UTC().Before(pastDueSince.Add(PastDuePaymentGraceDuration))
}

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
		if err := db.Select("subscription_status, trial_ends_at, clinic_id, past_due_since").First(&doctor, doctorID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "No se pudo verificar tu suscripción"})
			return
		}

		// Doctor de clínica: su acceso depende de la suscripción de la
		// clínica (consultada en vivo, no de una copia guardada en el
		// doctor) — el plan de clínica no tiene prueba gratis automática,
		// pero el superadmin puede otorgar una a mano (ver
		// handlers.GrantClinicFreeAccess), que deja a la clínica en
		// "trialing" igual que un doctor individual.
		if doctor.ClinicID != nil {
			var clinic models.Clinic
			if err := db.Select("subscription_status, trial_ends_at, past_due_since").First(&clinic, *doctor.ClinicID).Error; err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "No se pudo verificar la suscripción de tu clínica"})
				return
			}
			if clinic.SubscriptionStatus == "active" {
				c.Next()
				return
			}
			if clinic.SubscriptionStatus == "trialing" && clinic.TrialEndsAt != nil && time.Now().UTC().Before(*clinic.TrialEndsAt) {
				c.Next()
				return
			}
			if clinic.SubscriptionStatus == "past_due" && withinPastDueGrace(clinic.PastDueSince) {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
				"error":   "subscription_required",
				"message": "La suscripción de tu clínica no está activa. Pide al dueño de la clínica que la revise.",
			})
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

		if doctor.SubscriptionStatus == "past_due" && withinPastDueGrace(doctor.PastDueSince) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
			"error":   "subscription_required",
			"message": "Tu periodo de prueba terminó. Activa tu suscripción para seguir usando ProPatient Clinic.",
		})
	}
}
