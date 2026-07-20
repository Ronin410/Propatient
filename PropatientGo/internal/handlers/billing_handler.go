package handlers

import (
	"encoding/json"
	"io"
	"log"
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

// activateClinicSubscription confirma el pago del plan base de una clínica
// nueva (ver handlers.CreateClinic, que deliberadamente no toca
// Doctor.ClinicID hasta este punto) y cancela la suscripción individual del
// dueño si tenía una — mismo criterio que AcceptClinicInvite, para que no le
// sigan cobrando dos veces una vez que la clínica ya lo cubre.
func activateClinicSubscription(c *gin.Context, db *gorm.DB, client billing.Client, clinicID uint, sess *stripe.CheckoutSession) {
	updates := map[string]interface{}{"subscription_status": "active"}
	if sess.Customer != nil {
		updates["stripe_customer_id"] = sess.Customer.ID
	}
	if sess.Subscription != nil {
		updates["stripe_subscription_id"] = sess.Subscription.ID
	}
	result := db.Model(&models.Clinic{}).Where("id = ?", clinicID).Updates(updates)
	if result.Error != nil {
		log.Printf("⚠️ Webhook checkout.session.completed: error al actualizar clínica %d: %v", clinicID, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		log.Printf("⚠️ Webhook checkout.session.completed: no existe ninguna clínica con id %d", clinicID)
		return
	}

	var clinic models.Clinic
	if err := db.First(&clinic, clinicID).Error; err != nil {
		log.Printf("⚠️ Webhook checkout.session.completed: no se pudo releer la clínica %d recién activada: %v", clinicID, err)
		return
	}
	var owner models.Doctor
	if err := db.First(&owner, clinic.OwnerDoctorID).Error; err != nil {
		log.Printf("⚠️ Webhook checkout.session.completed: no se encontró al dueño (doctor %d) de la clínica %d: %v", clinic.OwnerDoctorID, clinicID, err)
		return
	}

	if client != nil && owner.StripeSubscriptionID != "" {
		if err := client.CancelSubscription(c.Request.Context(), owner.StripeSubscriptionID); err != nil {
			log.Printf("⚠️ Webhook checkout.session.completed: no se pudo cancelar la suscripción individual del doctor %d al activar su clínica %d: %v", owner.ID, clinicID, err)
		}
	}

	ownerUpdates := map[string]interface{}{
		"clinic_id":              clinic.ID,
		"is_clinic_owner":        true,
		"subscription_status":    "active",
		"stripe_subscription_id": "",
	}
	if err := db.Model(&owner).Updates(ownerUpdates).Error; err != nil {
		log.Printf("⚠️ Webhook checkout.session.completed: error al vincular al dueño (doctor %d) con su clínica %d: %v", owner.ID, clinicID, err)
		return
	}
	log.Printf("✅ Clínica %d activada, dueño = doctor %d", clinicID, owner.ID)
}

// StripeWebhook recibe los eventos que Stripe manda a medida que cambia el
// estado de una suscripción. Ruta pública (Stripe la llama sin sesión de
// usuario); la verificación de firma con el webhook signing secret es lo
// único que confirma que la petición viene realmente de Stripe.
func StripeWebhook(db *gorm.DB, client billing.Client, cfg billing.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.WebhookSecret == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Webhook no configurado"})
			return
		}

		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("⚠️ Webhook de Stripe: no se pudo leer el body: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudo leer el cuerpo de la petición"})
			return
		}

		// IgnoreAPIVersionMismatch: true — la cuenta de Stripe del doctor puede
		// estar en una versión de API más nueva que la que stripe-go 81.4.0
		// reconoce (stripe-go rechaza el evento por defecto ante cualquier
		// desajuste, aunque sea solo de versión). Los campos que de verdad
		// usamos del payload (client_reference_id, customer.id,
		// subscription.id/status) son estables entre versiones de la API de
		// Stripe, así que ignorar el desajuste es seguro aquí. La firma en sí
		// SIEMPRE se sigue verificando — esto no relaja esa parte.
		event, err := webhook.ConstructEventWithOptions(
			payload, c.GetHeader("Stripe-Signature"), cfg.WebhookSecret,
			webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
		)
		if err != nil {
			log.Printf("⚠️ Webhook de Stripe: firma inválida (revisa STRIPE_WEBHOOK_SECRET): %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Firma de Stripe inválida"})
			return
		}

		log.Printf("📩 Webhook de Stripe recibido: %s", event.Type)

		switch event.Type {
		case "checkout.session.completed":
			var sess stripe.CheckoutSession
			if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
				log.Printf("⚠️ Webhook checkout.session.completed: no se pudo decodificar el payload: %v", err)
				break
			}
			if clinicID, ok := billing.ParseClinicClientReferenceID(sess.ClientReferenceID); ok {
				activateClinicSubscription(c, db, client, clinicID, &sess)
				break
			}
			doctorID, err := strconv.ParseUint(sess.ClientReferenceID, 10, 64)
			if err != nil {
				log.Printf("⚠️ Webhook checkout.session.completed: client_reference_id inválido (%q): %v", sess.ClientReferenceID, err)
				break
			}
			updates := map[string]interface{}{"subscription_status": "active"}
			if sess.Customer != nil {
				updates["stripe_customer_id"] = sess.Customer.ID
			}
			if sess.Subscription != nil {
				updates["stripe_subscription_id"] = sess.Subscription.ID
			}
			result := db.Model(&models.Doctor{}).Where("id = ?", doctorID).Updates(updates)
			if result.Error != nil {
				log.Printf("⚠️ Webhook checkout.session.completed: error al actualizar doctor %d: %v", doctorID, result.Error)
			} else if result.RowsAffected == 0 {
				log.Printf("⚠️ Webhook checkout.session.completed: no existe ningún doctor con id %d", doctorID)
			} else {
				log.Printf("✅ Suscripción activada para el doctor %d", doctorID)
			}

		case "customer.subscription.updated":
			var sub stripe.Subscription
			if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
				log.Printf("⚠️ Webhook customer.subscription.updated: no se pudo decodificar el payload: %v", err)
				break
			}
			newStatus := mapStripeSubscriptionStatus(sub.Status)
			result := db.Model(&models.Doctor{}).
				Where("stripe_subscription_id = ?", sub.ID).
				Update("subscription_status", newStatus)
			if result.Error == nil && result.RowsAffected == 0 {
				// No es la suscripción individual de ningún doctor — puede
				// ser la de una clínica (ver activateClinicSubscription).
				result = db.Model(&models.Clinic{}).
					Where("stripe_subscription_id = ?", sub.ID).
					Update("subscription_status", newStatus)
			}
			if result.Error != nil {
				log.Printf("⚠️ Webhook customer.subscription.updated: error al actualizar suscripción %s: %v", sub.ID, result.Error)
			} else if result.RowsAffected == 0 {
				log.Printf("⚠️ Webhook customer.subscription.updated: ningún doctor ni clínica tiene stripe_subscription_id=%s", sub.ID)
			} else {
				log.Printf("✅ Suscripción %s actualizada a %q", sub.ID, newStatus)
			}

		case "customer.subscription.deleted":
			var sub stripe.Subscription
			if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
				log.Printf("⚠️ Webhook customer.subscription.deleted: no se pudo decodificar el payload: %v", err)
				break
			}
			result := db.Model(&models.Doctor{}).
				Where("stripe_subscription_id = ?", sub.ID).
				Update("subscription_status", "canceled")
			if result.Error == nil && result.RowsAffected == 0 {
				result = db.Model(&models.Clinic{}).
					Where("stripe_subscription_id = ?", sub.ID).
					Update("subscription_status", "canceled")
			}
			if result.Error != nil {
				log.Printf("⚠️ Webhook customer.subscription.deleted: error al cancelar suscripción %s: %v", sub.ID, result.Error)
			} else if result.RowsAffected == 0 {
				log.Printf("⚠️ Webhook customer.subscription.deleted: ningún doctor ni clínica tiene stripe_subscription_id=%s", sub.ID)
			}
		}

		c.JSON(http.StatusOK, gin.H{"received": true})
	}
}
