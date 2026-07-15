package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"propatient-api/internal/billing"
	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"gorm.io/gorm"
)

// GetBillingStatus expone el estado de suscripción del doctor para que el
// frontend pueda mostrar el banner de prueba/pago sin depender de que
// alguna otra ruta le devuelva un 402.
func GetBillingStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.Select("subscription_status, trial_ends_at, stripe_customer_id").First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"subscriptionStatus": doctor.SubscriptionStatus,
			"trialEndsAt":        doctor.TrialEndsAt,
			"hasPaymentMethod":   doctor.StripeCustomerID != "",
		})
	}
}

// CreateCheckoutSession arma la URL de Stripe Checkout para que el doctor
// suscriba su consultorio. Debe quedar fuera de RequireActiveSubscription:
// un doctor con la prueba vencida necesita poder llegar aquí para pagar.
func CreateCheckoutSession(db *gorm.DB, client billing.Client, cfg billing.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if client == nil || !cfg.IsConfigured() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "El cobro de suscripción no está configurado en el servidor."})
			return
		}

		doctorID := c.MustGet("doctorID").(uint)
		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		base := frontendRedirectBase()
		url, err := client.CreateCheckoutSession(c.Request.Context(), billing.CheckoutParams{
			DoctorID:           doctorID,
			CustomerEmail:      doctor.Email,
			ExistingCustomerID: doctor.StripeCustomerID,
			SuccessURL:         base + "/billing?checkout=success",
			CancelURL:          base + "/billing?checkout=cancelled",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo iniciar el proceso de pago"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}

// CreatePortalSession arma la URL del Customer Portal de Stripe, donde el
// doctor puede actualizar su tarjeta o cancelar sin que tengamos que
// construir esa UI nosotros.
func CreatePortalSession(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if client == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "El cobro de suscripción no está configurado en el servidor."})
			return
		}

		doctorID := c.MustGet("doctorID").(uint)
		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.StripeCustomerID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Aún no tienes una suscripción con la que abrir el portal de pagos."})
			return
		}

		url, err := client.CreatePortalSession(c.Request.Context(), doctor.StripeCustomerID, frontendRedirectBase()+"/billing")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo abrir el portal de facturación"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}

// mapStripeSubscriptionStatus reduce los ~8 estados posibles de una
// Subscription de Stripe a los 4 que usa SubscriptionStatus en el modelo.
func mapStripeSubscriptionStatus(s stripe.SubscriptionStatus) string {
	switch s {
	case stripe.SubscriptionStatusActive, stripe.SubscriptionStatusTrialing:
		return "active"
	case stripe.SubscriptionStatusPastDue, stripe.SubscriptionStatusUnpaid, stripe.SubscriptionStatusIncomplete:
		return "past_due"
	default: // canceled, incomplete_expired, paused
		return "canceled"
	}
}

// StripeWebhook recibe los eventos que Stripe manda a medida que cambia el
// estado de una suscripción. Ruta pública (Stripe la llama sin sesión de
// usuario); la verificación de firma con el webhook signing secret es lo
// único que confirma que la petición viene realmente de Stripe.
func StripeWebhook(db *gorm.DB, cfg billing.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.WebhookSecret == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Webhook no configurado"})
			return
		}

		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudo leer el cuerpo de la petición"})
			return
		}

		event, err := webhook.ConstructEvent(payload, c.GetHeader("Stripe-Signature"), cfg.WebhookSecret)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Firma de Stripe inválida"})
			return
		}

		switch event.Type {
		case "checkout.session.completed":
			var sess stripe.CheckoutSession
			if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
				break
			}
			doctorID, err := strconv.ParseUint(sess.ClientReferenceID, 10, 64)
			if err != nil {
				break
			}
			updates := map[string]interface{}{"subscription_status": "active"}
			if sess.Customer != nil {
				updates["stripe_customer_id"] = sess.Customer.ID
			}
			if sess.Subscription != nil {
				updates["stripe_subscription_id"] = sess.Subscription.ID
			}
			db.Model(&models.Doctor{}).Where("id = ?", doctorID).Updates(updates)

		case "customer.subscription.updated":
			var sub stripe.Subscription
			if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
				break
			}
			db.Model(&models.Doctor{}).
				Where("stripe_subscription_id = ?", sub.ID).
				Update("subscription_status", mapStripeSubscriptionStatus(sub.Status))

		case "customer.subscription.deleted":
			var sub stripe.Subscription
			if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
				break
			}
			db.Model(&models.Doctor{}).
				Where("stripe_subscription_id = ?", sub.ID).
				Update("subscription_status", "canceled")
		}

		c.JSON(http.StatusOK, gin.H{"received": true})
	}
}
