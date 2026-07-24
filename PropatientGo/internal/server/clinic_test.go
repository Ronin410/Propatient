package server_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"propatient-api/internal/billing"
	"propatient-api/internal/geocoding"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

// clinicBillingConfig arma una billing.Config con el plan de clínica
// configurado (además de la individual, ya que IsClinicConfigured también
// exige IsConfigured()) — mismo criterio que el resto de integraciones
// opcionales de la app.
func clinicBillingConfig() billing.Config {
	return billing.Config{
		SecretKey:          "sk_test_mock",
		PriceID:            "price_individual_mock",
		ClinicBasePriceID:  "price_clinic_base_mock",
		ClinicExtraPriceID: "price_clinic_extra_mock",
	}
}

// TestClinic_CreateClinic_NotConfigured_Returns503 confirma que sin
// STRIPE_CLINIC_BASE_PRICE_ID/STRIPE_CLINIC_EXTRA_DOCTOR_PRICE_ID el plan de
// clínica se comporta como no configurado, igual que el resto de
// integraciones opcionales de la app.
func TestClinic_CreateClinic_NotConfigured_Returns503(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_clinic_noconfig", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/clinic", token, map[string]any{"name": "Clínica Test"})
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestClinic_CreateClinic_CreatesOrphanRow_DoctorNotLinkedYet confirma el
// punto central del diseño: CreateClinic arma el Checkout pero deja al
// doctor sin tocar hasta que el webhook confirme el pago (ver
// activateClinicSubscription) — así un Checkout abandonado no lo deja sin
// acceso a su cuenta individual.
func TestClinic_CreateClinic_CreatesOrphanRow_DoctorNotLinkedYet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mock := newMockBillingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), clinicBillingConfig(), mock, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_clinic_create", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/clinic", token, map[string]any{"name": "Clínica Test"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.NotEmpty(t, body["url"])

	var clinic models.Clinic
	require.NoError(t, db.Where("owner_doctor_id = ?", doc.ID).First(&clinic).Error)
	assert.Equal(t, "Clínica Test", clinic.Name)
	assert.Equal(t, "incomplete", clinic.SubscriptionStatus)

	var reloaded models.Doctor
	require.NoError(t, db.First(&reloaded, doc.ID).Error)
	assert.Nil(t, reloaded.ClinicID, "el doctor no debe quedar vinculado hasta que el webhook confirme el pago")

	require.Len(t, mock.clinicCheckoutCalls, 1)
	assert.Equal(t, clinic.ID, mock.clinicCheckoutCalls[0].ClinicID)
}

// TestClinic_CreateClinic_AlreadyInClinic_Returns409 evita que un doctor
// que ya pertenece a una clínica cree otra por encima.
func TestClinic_CreateClinic_AlreadyInClinic_Returns409(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mock := newMockBillingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), clinicBillingConfig(), mock, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_clinic_already", "password123")
	clinic := models.Clinic{Name: "Otra Clínica", OwnerDoctorID: doc.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/clinic", token, map[string]any{"name": "Clínica Nueva"})
	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestClinic_GetClinic_OwnerSeesDoctorsAndBreakdown confirma la vista del
// dueño: su lista de doctores y el desglose informativo de costo.
func TestClinic_GetClinic_OwnerSeesDoctorsAndBreakdown(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_get", "password123")
	clinic := models.Clinic{Name: "Clínica Central", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	member := testutil.CreateTestDoctor(t, db, "doc_clinic_member_get", "password123")
	require.NoError(t, db.Model(&member).Update("clinic_id", clinic.ID).Error)

	token := testutil.TokenFor(t, owner.ID, owner.Username)
	w := doRequest(t, router, http.MethodGet, "/api/clinic", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	body := decodeJSON(t, w)
	assert.Equal(t, "Clínica Central", body["name"])
	assert.Equal(t, true, body["isOwner"])
	assert.Equal(t, float64(0), body["extraDoctors"])
	doctors, ok := body["doctors"].([]any)
	require.True(t, ok)
	assert.Len(t, doctors, 2)
}

// TestClinic_GetClinic_NonMember_Returns404 confirma que un doctor sin
// clínica no ve nada.
func TestClinic_GetClinic_NonMember_Returns404(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_clinic_nonmember", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/clinic", token, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestClinic_InviteDoctorToClinic_OnlyOwnerCanInvite confirma que un
// miembro (no dueño) no puede invitar a nadie.
func TestClinic_InviteDoctorToClinic_OnlyOwnerCanInvite(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_inv", "password123")
	clinic := models.Clinic{Name: "Clínica X", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	member := testutil.CreateTestDoctor(t, db, "doc_clinic_member_inv", "password123")
	require.NoError(t, db.Model(&member).Update("clinic_id", clinic.ID).Error)

	invitee := testutil.CreateTestDoctor(t, db, "doc_clinic_invitee_a", "password123")

	memberToken := testutil.TokenFor(t, member.ID, member.Username)
	w := doRequest(t, router, http.MethodPost, "/api/clinic/invite", memberToken, map[string]any{"email": invitee.Email})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestClinic_InviteDoctorToClinic_InviteeMustHaveAccount confirma que solo
// se puede invitar a un doctor que ya tiene cuenta en ProPatient.
func TestClinic_InviteDoctorToClinic_InviteeMustHaveAccount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_inv2", "password123")
	clinic := models.Clinic{Name: "Clínica X", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	token := testutil.TokenFor(t, owner.ID, owner.Username)
	w := doRequest(t, router, http.MethodPost, "/api/clinic/invite", token, map[string]any{"email": "nadie@nunca-existio.local"})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestClinic_InviteDoctorToClinic_Success_SetsPendingFields confirma que
// invitar a un doctor existente le guarda la invitación pendiente sin
// tocar todavía su ClinicID (eso lo hace AcceptClinicInvite).
func TestClinic_InviteDoctorToClinic_Success_SetsPendingFields(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_inv3", "password123")
	clinic := models.Clinic{Name: "Clínica X", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	invitee := testutil.CreateTestDoctor(t, db, "doc_clinic_invitee_b", "password123")

	token := testutil.TokenFor(t, owner.ID, owner.Username)
	w := doRequest(t, router, http.MethodPost, "/api/clinic/invite", token, map[string]any{"email": invitee.Email})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var reloaded models.Doctor
	require.NoError(t, db.First(&reloaded, invitee.ID).Error)
	require.NotNil(t, reloaded.PendingClinicID)
	assert.Equal(t, clinic.ID, *reloaded.PendingClinicID)
	assert.NotEmpty(t, reloaded.ClinicInviteToken)
	require.NotNil(t, reloaded.ClinicInviteTokenExpiresAt)
	assert.True(t, reloaded.ClinicInviteTokenExpiresAt.After(time.Now().UTC()))
}

// TestClinic_GetClinicInvite_ShowsClinicAndOwnerName_WithoutAccepting
// confirma que consultar la invitación no la acepta — es una vista previa
// (usada por la pantalla de "aceptar invitación" antes de confirmar).
func TestClinic_GetClinicInvite_ShowsClinicAndOwnerName_WithoutAccepting(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_prev", "password123")
	require.NoError(t, db.Model(&owner).Update("full_name", "Dra. Ana Preview").Error)
	clinic := models.Clinic{Name: "Clínica Preview", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)

	invitee := testutil.CreateTestDoctor(t, db, "doc_clinic_invitee_prev", "password123")
	expires := time.Now().UTC().Add(72 * time.Hour)
	require.NoError(t, db.Model(&invitee).Updates(map[string]any{
		"pending_clinic_id":              clinic.ID,
		"clinic_invite_token":            "preview-token",
		"clinic_invite_token_expires_at": expires,
	}).Error)

	token := testutil.TokenFor(t, invitee.ID, invitee.Username)
	w := doRequest(t, router, http.MethodGet, "/api/clinic/invitations/preview-token", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, "Clínica Preview", body["clinicName"])
	assert.Equal(t, "Dra. Ana Preview", body["ownerName"])

	var reloaded models.Doctor
	require.NoError(t, db.First(&reloaded, invitee.ID).Error)
	assert.Nil(t, reloaded.ClinicID, "consultar la invitación no debe aceptarla")
}

// TestClinic_AcceptClinicInvite_Success_CancelsIndividualSubscription
// confirma el flujo central: el doctor invitado se une, su suscripción
// individual (si tenía una) se cancela, y el conteo de doctores extra de
// la clínica se sincroniza con Stripe.
func TestClinic_AcceptClinicInvite_Success_CancelsIndividualSubscription(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mock := newMockBillingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), clinicBillingConfig(), mock, geocoding.NewClient(), nil, nil)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_acc", "password123")
	clinic := models.Clinic{
		Name:                 "Clínica Y",
		OwnerDoctorID:        owner.ID,
		SubscriptionStatus:   "active",
		StripeSubscriptionID: "sub_clinic_y",
	}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	// Ya hay 5 doctores en la clínica (el dueño + 4 más) — el sexto que se
	// une debe disparar el concepto de "doctor extra" (cantidad 1).
	for i := 0; i < 4; i++ {
		filler := testutil.CreateTestDoctor(t, db, "doc_clinic_filler_"+string(rune('a'+i)), "password123")
		require.NoError(t, db.Model(&filler).Update("clinic_id", clinic.ID).Error)
	}

	invitee := testutil.CreateTestDoctor(t, db, "doc_clinic_invitee_c", "password123")
	expires := time.Now().UTC().Add(72 * time.Hour)
	require.NoError(t, db.Model(&invitee).Updates(map[string]any{
		"pending_clinic_id":              clinic.ID,
		"clinic_invite_token":            "test-token-123",
		"clinic_invite_token_expires_at": expires,
		"stripe_subscription_id":         "sub_individual_invitee",
	}).Error)

	token := testutil.TokenFor(t, invitee.ID, invitee.Username)
	w := doRequest(t, router, http.MethodPost, "/api/clinic/invitations/test-token-123/accept", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.True(t, mock.wasCanceled("sub_individual_invitee"))

	var reloaded models.Doctor
	require.NoError(t, db.First(&reloaded, invitee.ID).Error)
	require.NotNil(t, reloaded.ClinicID)
	assert.Equal(t, clinic.ID, *reloaded.ClinicID)
	assert.False(t, reloaded.IsClinicOwner)
	assert.Nil(t, reloaded.PendingClinicID)
	assert.Empty(t, reloaded.ClinicInviteToken)
	assert.Equal(t, "active", reloaded.SubscriptionStatus)
	assert.Empty(t, reloaded.StripeSubscriptionID)

	call, ok := mock.lastExtraQtyCall()
	require.True(t, ok)
	assert.Equal(t, "sub_clinic_y", call.SubscriptionID)
	assert.Equal(t, int64(1), call.Quantity)
}

// TestClinic_AcceptClinicInvite_ExpiredToken_Returns410 confirma que una
// invitación vieja ya no se puede aceptar.
func TestClinic_AcceptClinicInvite_ExpiredToken_Returns410(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_exp", "password123")
	clinic := models.Clinic{Name: "Clínica Z", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)

	invitee := testutil.CreateTestDoctor(t, db, "doc_clinic_invitee_d", "password123")
	expired := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&invitee).Updates(map[string]any{
		"pending_clinic_id":              clinic.ID,
		"clinic_invite_token":            "expired-token",
		"clinic_invite_token_expires_at": expired,
	}).Error)

	token := testutil.TokenFor(t, invitee.ID, invitee.Username)
	w := doRequest(t, router, http.MethodPost, "/api/clinic/invitations/expired-token/accept", token, nil)
	assert.Equal(t, http.StatusGone, w.Code)
}

// TestClinic_AcceptClinicInvite_WrongToken_Returns404 confirma que el
// token debe coincidir exactamente con el guardado para ese doctor.
func TestClinic_AcceptClinicInvite_WrongToken_Returns404(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	invitee := testutil.CreateTestDoctor(t, db, "doc_clinic_invitee_e", "password123")
	token := testutil.TokenFor(t, invitee.ID, invitee.Username)
	w := doRequest(t, router, http.MethodPost, "/api/clinic/invitations/no-such-token/accept", token, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestClinic_RemoveDoctorFromClinic_OnlyOwner_Returns403 confirma que un
// miembro no puede quitar a otro doctor.
func TestClinic_RemoveDoctorFromClinic_OnlyOwner_Returns403(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_rm1", "password123")
	clinic := models.Clinic{Name: "Clínica R", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	member := testutil.CreateTestDoctor(t, db, "doc_clinic_member_rm1", "password123")
	require.NoError(t, db.Model(&member).Update("clinic_id", clinic.ID).Error)
	otherMember := testutil.CreateTestDoctor(t, db, "doc_clinic_member_rm2", "password123")
	require.NoError(t, db.Model(&otherMember).Update("clinic_id", clinic.ID).Error)

	memberToken := testutil.TokenFor(t, member.ID, member.Username)
	w := doRequest(t, router, http.MethodDelete, "/api/clinic/doctors/"+itoa(otherMember.ID), memberToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestClinic_RemoveDoctorFromClinic_OwnerCannotRemoveSelf confirma que el
// dueño no puede quitarse a sí mismo por esta ruta.
func TestClinic_RemoveDoctorFromClinic_OwnerCannotRemoveSelf(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_rm3", "password123")
	clinic := models.Clinic{Name: "Clínica R2", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	token := testutil.TokenFor(t, owner.ID, owner.Username)
	w := doRequest(t, router, http.MethodDelete, "/api/clinic/doctors/"+itoa(owner.ID), token, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestClinic_RemoveDoctorFromClinic_Success confirma que quitar a un
// doctor libera su ClinicID y ajusta el conteo extra de Stripe hacia abajo.
func TestClinic_RemoveDoctorFromClinic_Success(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mock := newMockBillingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), clinicBillingConfig(), mock, geocoding.NewClient(), nil, nil)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_rm4", "password123")
	clinic := models.Clinic{
		Name:                 "Clínica R3",
		OwnerDoctorID:        owner.ID,
		SubscriptionStatus:   "active",
		StripeSubscriptionID: "sub_clinic_r3",
		StripeExtraItemID:    "mock_extra_item_existing",
	}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	// 6 doctores en total (dueño + 5) para que quitar a uno los deje
	// justo en el límite del plan base (5) y el concepto extra deba
	// borrarse (cantidad 0).
	var target models.Doctor
	for i := 0; i < 5; i++ {
		d := testutil.CreateTestDoctor(t, db, "doc_clinic_filler2_"+string(rune('a'+i)), "password123")
		require.NoError(t, db.Model(&d).Update("clinic_id", clinic.ID).Error)
		target = d
	}

	token := testutil.TokenFor(t, owner.ID, owner.Username)
	w := doRequest(t, router, http.MethodDelete, "/api/clinic/doctors/"+itoa(target.ID), token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var reloaded models.Doctor
	require.NoError(t, db.First(&reloaded, target.ID).Error)
	assert.Nil(t, reloaded.ClinicID)
	assert.Equal(t, "canceled", reloaded.SubscriptionStatus)

	call, ok := mock.lastExtraQtyCall()
	require.True(t, ok)
	assert.Equal(t, int64(0), call.Quantity)
}

// TestClinic_Middleware_BlocksMember_WhenClinicSubscriptionNotActive
// confirma que el acceso de un doctor de clínica depende de la
// suscripción de la clínica, no de la suya propia.
func TestClinic_Middleware_BlocksMember_WhenClinicSubscriptionNotActive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_mw1", "password123")
	clinic := models.Clinic{Name: "Clínica Incompleta", OwnerDoctorID: owner.ID, SubscriptionStatus: "incomplete"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	token := testutil.TokenFor(t, owner.ID, owner.Username)
	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusPaymentRequired, w.Code)
}

// TestClinic_Middleware_AllowsMember_WhenClinicActive_EvenIfOwnTrialExpired
// confirma el caso central del middleware extendido: la clínica activa
// cubre a un doctor cuya propia prueba individual ya venció.
func TestClinic_Middleware_AllowsMember_WhenClinicActive_EvenIfOwnTrialExpired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_mw2", "password123")
	expired := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&owner).Update("trial_ends_at", expired).Error)

	clinic := models.Clinic{Name: "Clínica Activa", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	token := testutil.TokenFor(t, owner.ID, owner.Username)
	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestClinic_UpdateClinicLocation_OnlyOwnerCanUpdate confirma que un
// doctor miembro (no dueño) no puede cambiar la ubicación única de la
// clínica — el consultorio de clínica no es "movible" por cada doctor.
func TestClinic_UpdateClinicLocation_OnlyOwnerCanUpdate(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_loc_owner1", "password123")
	clinic := models.Clinic{Name: "Clínica Ubicación", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	member := testutil.CreateTestDoctor(t, db, "doc_clinic_loc_member1", "password123")
	require.NoError(t, db.Model(&member).Update("clinic_id", clinic.ID).Error)

	token := testutil.TokenFor(t, member.ID, member.Username)
	w := doRequest(t, router, http.MethodPut, "/api/clinic/location", token, map[string]any{
		"address": "Calle Falsa 123", "latitude": "20.0", "longitude": "-103.0",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestClinic_UpdateClinicLocation_NonClinicDoctor_Returns403 confirma que
// un doctor independiente (sin clínica) tampoco puede llamar este endpoint.
func TestClinic_UpdateClinicLocation_NonClinicDoctor_Returns403(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_clinic_loc_indep1", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPut, "/api/clinic/location", token, map[string]any{
		"address": "Calle Falsa 123", "latitude": "20.0", "longitude": "-103.0",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestClinic_UpdateClinicLocation_Success_PersistsAddressAndCoordinates
// confirma el camino feliz: el dueño guarda la dirección/coordenadas, y
// quedan persistidas en la fila de la clínica.
func TestClinic_UpdateClinicLocation_Success_PersistsAddressAndCoordinates(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_loc_owner2", "password123")
	clinic := models.Clinic{Name: "Clínica Ubicación 2", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	token := testutil.TokenFor(t, owner.ID, owner.Username)
	w := doRequest(t, router, http.MethodPut, "/api/clinic/location", token, map[string]any{
		"address": "Av. Siempre Viva 742", "latitude": "20.6597", "longitude": "-103.3496",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var updated models.Clinic
	require.NoError(t, db.First(&updated, clinic.ID).Error)
	assert.Equal(t, "Av. Siempre Viva 742", updated.Address)
	require.NotNil(t, updated.Latitude)
	require.NotNil(t, updated.Longitude)
	assert.InDelta(t, 20.6597, *updated.Latitude, 0.0001)
	assert.InDelta(t, -103.3496, *updated.Longitude, 0.0001)
}
