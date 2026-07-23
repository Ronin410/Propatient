package server_test

import (
	"net/http"
	"testing"

	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReferral_UpdateProfile_AppliesValidCode confirma que capturar un
// código de invitación válido al completar el perfil crea la fila
// Referral "pending" y lo refleja en la respuesta (referralCodeApplied).
func TestReferral_UpdateProfile_AppliesValidCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	referrer := testutil.CreateTestDoctor(t, db, "doc_ref_int_referrer", "password123")
	require.NoError(t, db.Model(&referrer).Update("referral_code", "INTCODE1").Error)

	referred := testutil.CreateTestDoctor(t, db, "doc_ref_int_referred", "password123")
	token := testutil.TokenFor(t, referred.ID, referred.Username)

	payload := map[string]any{
		"fullName":         "Dr. Referido de Prueba",
		"medicalSpecialty": "Medicina General",
		"phone":            "5555555555",
		"birthDate":        "1990-01-01",
		"referralCode":     "intcode1",
	}
	w := doRequest(t, router, http.MethodPost, "/api/user/update-profile", token, payload)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, true, body["referralCodeApplied"])

	var ref models.Referral
	require.NoError(t, db.Where("referred_doctor_id = ?", referred.ID).First(&ref).Error)
	assert.Equal(t, referrer.ID, ref.ReferrerDoctorID)
	assert.Equal(t, "pending", ref.Status)
}

// TestReferral_UpdateProfile_InvalidCodeNeverBlocksSave confirma que un
// código inválido no impide guardar el resto del perfil — solo se
// refleja como no aplicado.
func TestReferral_UpdateProfile_InvalidCodeNeverBlocksSave(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_ref_int_invalid", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	payload := map[string]any{
		"fullName":         "Dr. Sin Código Válido",
		"medicalSpecialty": "Pediatría",
		"phone":            "5555555556",
		"birthDate":        "1990-01-01",
		"referralCode":     "NOEXISTE",
	}
	w := doRequest(t, router, http.MethodPost, "/api/user/update-profile", token, payload)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, false, body["referralCodeApplied"])

	var updated models.Doctor
	require.NoError(t, db.First(&updated, doc.ID).Error)
	assert.Equal(t, "Dr. Sin Código Válido", updated.FullName, "el resto del perfil debe guardarse aunque el código no sea válido")
}

// TestReferral_GetReferralCode_ForbiddenWithoutActiveSubscription
// confirma que un doctor en prueba (no "active") no puede ver ni
// compartir su código todavía.
func TestReferral_GetReferralCode_ForbiddenWithoutActiveSubscription(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_ref_int_notactive", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/referral-code", token, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestReferral_GetReferralCode_ActiveSubscription_ReturnsCodeAndCounts
// confirma el camino feliz: con suscripción activa, el endpoint trae el
// código propio y los contadores correctos.
func TestReferral_GetReferralCode_ActiveSubscription_ReturnsCodeAndCounts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_ref_int_active", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"subscription_status": "active",
		"referral_code":       "ACTIVE01",
	}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	// Un referido ya premiado y otro todavía pendiente, para confirmar
	// que los contadores distinguen entre ambos.
	other1 := testutil.CreateTestDoctor(t, db, "doc_ref_int_active_r1", "password123")
	other2 := testutil.CreateTestDoctor(t, db, "doc_ref_int_active_r2", "password123")
	require.NoError(t, db.Create(&models.Referral{ReferrerDoctorID: doc.ID, ReferredDoctorID: other1.ID, Status: "rewarded"}).Error)
	require.NoError(t, db.Create(&models.Referral{ReferrerDoctorID: doc.ID, ReferredDoctorID: other2.ID, Status: "pending"}).Error)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/referral-code", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, "ACTIVE01", body["code"])
	assert.EqualValues(t, 2, body["totalReferrals"])
	assert.EqualValues(t, 1, body["rewardedReferrals"])
	assert.EqualValues(t, 8, body["remainingSlots"])
	assert.EqualValues(t, models.MaxReferralsPerDoctor, body["maxReferrals"])
}

// TestReferral_GetReferralCode_ClinicDoctorForbidden confirma que el
// mecanismo no aplica a doctores de clínica (fuera de alcance de esta
// primera versión — ver internal/referral/referral.go).
func TestReferral_GetReferralCode_ClinicDoctorForbidden(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_ref_int_clinic", "password123")
	clinic := models.Clinic{Name: "Clínica de prueba", OwnerDoctorID: doc.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"subscription_status": "active",
		"clinic_id":           clinic.ID,
	}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/referral-code", token, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestReferral_GetReferralCode_StaffForbidden confirma que solo el
// doctor gestiona/comparte su propio código, nunca el personal.
func TestReferral_GetReferralCode_StaffForbidden(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_ref_int_staffcheck", "password123")
	require.NoError(t, db.Model(&doc).Update("subscription_status", "active").Error)
	staff := testutil.CreateTestStaff(t, db, doc.ID, "personal@referral-test.local", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/referral-code", staffToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
