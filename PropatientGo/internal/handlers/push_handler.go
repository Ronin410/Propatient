package handlers

import (
	"net/http"

	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type pushSubscriptionRequest struct {
	Endpoint  string `json:"endpoint" binding:"required"`
	P256dhKey string `json:"p256dhKey" binding:"required"`
	AuthKey   string `json:"authKey" binding:"required"`
}

// SavePushSubscription guarda (o actualiza, si el Endpoint ya existía) la
// suscripción push del navegador/dispositivo desde el que se llama. Un
// doctor puede tener varias — cada dispositivo/navegador manda la suya.
func SavePushSubscription(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var req pushSubscriptionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		sub := models.PushSubscription{
			DoctorID:  doctorID,
			Endpoint:  req.Endpoint,
			P256dhKey: req.P256dhKey,
			AuthKey:   req.AuthKey,
		}
		// Upsert por Endpoint: si el mismo navegador ya se había suscrito
		// antes (ej. las llaves cambiaron, o quedó huérfana de otro doctor
		// en un dispositivo compartido), se actualiza en vez de duplicar.
		if err := db.Where(models.PushSubscription{Endpoint: req.Endpoint}).
			Assign(models.PushSubscription{DoctorID: doctorID, P256dhKey: req.P256dhKey, AuthKey: req.AuthKey}).
			FirstOrCreate(&sub).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar la suscripción"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Suscripción guardada"})
	}
}

type deletePushSubscriptionRequest struct {
	Endpoint string `json:"endpoint" binding:"required"`
}

// DeletePushSubscription quita la suscripción de este dispositivo (ej. el
// doctor desactivó las notificaciones desde el toggle en Perfil). Filtra
// también por doctorID para que un doctor no pueda borrar la suscripción
// de otro adivinando el endpoint.
func DeletePushSubscription(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var req deletePushSubscriptionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Where("doctor_id = ? AND endpoint = ?", doctorID, req.Endpoint).
			Delete(&models.PushSubscription{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo eliminar la suscripción"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Suscripción eliminada"})
	}
}
