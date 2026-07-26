// Package referral implementa el sistema de código de invitación entre
// doctores: un doctor con suscripción individual activa puede compartir
// su propio código; cuando otro doctor se registra usando ese código y
// SU PRIMER PAGO se completa, ambos ganan una semana extra gratis,
// acumulable, con un tope de models.MaxReferralsPerDoctor invitaciones
// por doctor.
//
// Paquete hoja aparte (no vive en internal/handlers) a propósito:
// internal/auth (donde se crea la cuenta del doctor) no puede importar
// internal/handlers sin generar un ciclo de imports (handlers ya importa
// auth para correo/formato de fechas/etc.), así que la lógica compartida
// entre auth, handlers y main.go vive aquí — mismo patrón que
// internal/billing, internal/audit, internal/whatsapp.
package referral

import (
	"context"
	"crypto/rand"
	"errors"
	"log"
	"strings"
	"time"

	"propatient-api/internal/billing"
	"propatient-api/internal/models"

	"gorm.io/gorm"
)

// codeAlphabet excluye 0/O/1/I: se prestan a confusión cuando un doctor
// dicta o teclea el código de otro. 8 caracteres de este alfabeto dan
// ~5x10^11 combinaciones — de sobra para no necesitar un índice único a
// nivel de base de datos (ver models.Doctor.ReferralCode).
const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
const codeLength = 8

// GenerateCode arma un código corto y único (reintenta contra la tabla
// en caso de colisión, igual que generateDoctorSlug hace con
// Doctor.PublicSlug) — no garantiza unicidad con una constraint de la
// base de datos, la garantiza aquí antes de asignarlo.
func GenerateCode(db *gorm.DB) (string, error) {
	for attempt := 0; attempt < 10; attempt++ {
		buf := make([]byte, codeLength)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		code := make([]byte, codeLength)
		for i, b := range buf {
			code[i] = codeAlphabet[int(b)%len(codeAlphabet)]
		}
		candidate := string(code)

		var count int64
		if err := db.Model(&models.Doctor{}).Where("referral_code = ?", candidate).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", errors.New("no se pudo generar un código de invitación único")
}

// ApplyCodeIfValid intenta enganchar al doctor recién registrado
// (referredDoctorID) con el dueño de rawCode. Nunca regresa error: un
// código vacío, inválido, propio, de alguien que ya tuvo una suscripción
// antes, o de un referidor que ya alcanzó su tope, simplemente no hace
// nada — aplicar un código de invitación jamás debe bloquear el registro
// de un doctor. Devuelve true solo si de verdad se enganchó.
func ApplyCodeIfValid(db *gorm.DB, referredDoctorID uint, rawCode string) bool {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" {
		return false
	}

	// El regalo solo aplica a la PRIMERA suscripción de quien captura el
	// código. StripeSubscriptionID nunca se borra al cancelar (ver
	// customer.subscription.deleted en el webhook, que solo cambia el
	// status), así que si ya tiene uno guardado significa que ya pagó
	// alguna vez — aunque hoy esté cancelada y esté por resuscribirse — y
	// el código deja de ser válido para él. Su propio código para
	// compartir con otros no se ve afectado por esto (ver
	// handlers.GetReferralCode, que no pasa por aquí).
	var referred models.Doctor
	if err := db.Select("id, stripe_subscription_id").First(&referred, referredDoctorID).Error; err != nil {
		return false
	}
	if referred.StripeSubscriptionID != "" {
		return false
	}

	var referrer models.Doctor
	if err := db.Select("id").Where("referral_code = ?", code).First(&referrer).Error; err != nil {
		return false
	}
	if referrer.ID == referredDoctorID {
		return false
	}

	var existingCount int64
	db.Model(&models.Referral{}).Where("referrer_doctor_id = ?", referrer.ID).Count(&existingCount)
	if existingCount >= models.MaxReferralsPerDoctor {
		return false
	}

	// El uniqueIndex en ReferredDoctorID rechaza silenciosamente un
	// segundo intento del mismo doctor referido (ya sea el mismo código
	// dos veces o uno distinto) — Create simplemente falla y se
	// interpreta como "no se aplicó", sin propagar el error.
	referral := models.Referral{ReferrerDoctorID: referrer.ID, ReferredDoctorID: referredDoctorID}
	if err := db.Create(&referral).Error; err != nil {
		return false
	}
	return true
}

// grantWeek le da 7 días extra a UN doctor: si está en prueba, se le
// suma directo a TrialEndsAt; si ya paga, se le pide a Stripe que
// recorra la fecha de cobro (ver billing.Client.ExtendSubscriptionByDays
// — RequireActiveSubscription no guarda ninguna fecha de vencimiento
// para un doctor "active", así que la única forma real de darle tiempo
// gratis es que Stripe mismo corra el cobro, no un campo local). Un
// doctor de clínica, o uno sin suscripción viva (past_due/canceled), se
// omite con un log — no hay nada seguro que extender ahí.
func grantWeek(ctx context.Context, db *gorm.DB, client billing.Client, doctorID uint) error {
	var doctor models.Doctor
	if err := db.Select("subscription_status, trial_ends_at, stripe_subscription_id, clinic_id").First(&doctor, doctorID).Error; err != nil {
		return err
	}

	if doctor.ClinicID != nil {
		log.Printf("ℹ️ Doctor %d es de clínica, no se le da semana de regalo (fuera de alcance)", doctorID)
		return nil
	}

	switch doctor.SubscriptionStatus {
	case "trialing":
		// Se acumula sobre lo que ya tenía (nunca se resetea a "ahora +7"):
		// si ya le quedaban 10 días de prueba, pasa a 17, no a 7.
		base := time.Now().UTC()
		if doctor.TrialEndsAt != nil {
			base = *doctor.TrialEndsAt
		}
		newEnd := base.Add(7 * 24 * time.Hour)
		return db.Model(&models.Doctor{}).Where("id = ?", doctorID).Update("trial_ends_at", newEnd).Error
	case "active":
		if doctor.StripeSubscriptionID == "" || client == nil {
			log.Printf("ℹ️ Doctor %d está activo pero sin suscripción de Stripe registrada, no se le puede dar semana de regalo", doctorID)
			return nil
		}
		return client.ExtendSubscriptionByDays(ctx, doctor.StripeSubscriptionID, 7)
	default:
		log.Printf("ℹ️ Doctor %d en estatus %q, no se le da semana de regalo", doctorID, doctor.SubscriptionStatus)
		return nil
	}
}

// GrantRewardIfPending se llama desde el webhook de Stripe cuando el
// PRIMER pago de un doctor se completa. Si ese doctor fue referido por
// alguien (fila "pending" en Referral), le da la semana de regalo a
// AMBOS lados y marca la referencia como "rewarded". Idempotente: si ya
// no hay ninguna fila "pending" para este doctor (ej. el webhook se
// reentrega), no hace nada — así nunca se duplica la recompensa.
func GrantRewardIfPending(ctx context.Context, db *gorm.DB, client billing.Client, referredDoctorID uint) {
	var ref models.Referral
	if err := db.Where("referred_doctor_id = ? AND status = ?", referredDoctorID, "pending").First(&ref).Error; err != nil {
		return // sin referencia pendiente: no es un doctor referido, o ya se premió antes.
	}

	if err := grantWeek(ctx, db, client, ref.ReferrerDoctorID); err != nil {
		log.Printf("⚠️ No se pudo dar la semana de regalo al referidor (doctor %d): %v", ref.ReferrerDoctorID, err)
	}
	if err := grantWeek(ctx, db, client, ref.ReferredDoctorID); err != nil {
		log.Printf("⚠️ No se pudo dar la semana de regalo al doctor referido (doctor %d): %v", ref.ReferredDoctorID, err)
	}

	now := time.Now().UTC()
	if err := db.Model(&models.Referral{}).Where("id = ?", ref.ID).Updates(map[string]any{
		"status":      "rewarded",
		"rewarded_at": now,
	}).Error; err != nil {
		log.Printf("⚠️ No se pudo marcar como premiada la referencia %d: %v", ref.ID, err)
	}
}
