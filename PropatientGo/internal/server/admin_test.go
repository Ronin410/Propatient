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
	"gorm.io/gorm"
)

func TestSuperAdminLogin_Success(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)
	testutil.CreateTestSuperAdmin(t, db, "admin_login", "clavesegura123")

	w := doRequest(t, router, http.MethodPost, "/api/admin/login", "", map[string]any{
		"username": "admin_login", "password": "clavesegura123",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp := decodeJSON(t, w)
	assert.NotEmpty(t, resp["token"])
}

func TestSuperAdminLogin_WrongPassword(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)
	testutil.CreateTestSuperAdmin(t, db, "admin_wrong", "clavesegura123")

	w := doRequest(t, router, http.MethodPost, "/api/admin/login", "", map[string]any{
		"username": "admin_wrong", "password": "incorrecta",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestDoctorToken_CannotAccessAdminRoutes confirma que un JWT de doctor
// normal (sin "role": "SUPERADMIN") no puede colarse en el panel interno,
// aunque sea técnicamente válido.
func TestDoctorToken_CannotAccessAdminRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_not_admin", "password123")
	doctorToken := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/admin/doctors/pending", doctorToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminRoutes_RequireToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodGet, "/api/admin/doctors/pending", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestSuperAdmin_CedulaReviewLifecycle cubre el flujo completo: un doctor
// termina su onboarding (queda CAPTURADA), el admin lo ve en la lista de
// pendientes, lo aprueba, y deja de aparecer en la lista.
func TestSuperAdmin_CedulaReviewLifecycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_pending_review", "password123")
	doc.CedulaValidated = "CAPTURADA"
	doc.LicenseNumber = "1234567"
	doc.IneDocumentPath = "identidad/ine_doctor_test.png"
	doc.CedulaDocumentPath = "identidad/cedula_doctor_test.png"
	require.NoError(t, db.Save(&doc).Error)

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_lifecycle", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	w := doRequest(t, router, http.MethodGet, "/api/admin/doctors/pending", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var pending []map[string]any
	decodeJSONList(t, w, &pending)
	require.Len(t, pending, 1)
	assert.Equal(t, "1234567", pending[0]["licenseNumber"])
	assert.NotEmpty(t, pending[0]["ineDocumentUrl"])
	// El documento de la cédula en sí (distinto del INE) también debe
	// verse en el panel, para que el revisor pueda cotejarlo contra el
	// número de licencia capturado.
	assert.NotEmpty(t, pending[0]["cedulaDocumentUrl"])

	docIDStr := strconv.FormatUint(uint64(doc.ID), 10)
	w = doRequest(t, router, http.MethodPut, "/api/admin/doctors/"+docIDStr+"/approve", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var reloaded struct{ CedulaValidated string }
	require.NoError(t, db.Table("doctors").Select("cedula_validated").Where("id = ?", doc.ID).Scan(&reloaded).Error)
	assert.Equal(t, "VALIDADA", reloaded.CedulaValidated)

	w = doRequest(t, router, http.MethodGet, "/api/admin/doctors/pending", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var pendingAfter []map[string]any
	decodeJSONList(t, w, &pendingAfter)
	assert.Len(t, pendingAfter, 0)
}

// TestApproveDoctorCedula_AutoJoinsPendingClinicEmailInvite cubre el flujo
// completo pedido por el usuario: un correo se invita a una clínica ANTES
// de tener cuenta (ver inviteClinicByEmail), esa persona se registra y
// pasa por la revisión de cédula normal, y en cuanto el superadmin la
// aprueba queda unida a la clínica sola — sin que el dueño tenga que
// volver a invitarla ni que ella confirme nada. También cancela su
// suscripción individual si ya tenía una.
func TestApproveDoctorCedula_AutoJoinsPendingClinicEmailInvite(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, nil, billing.Config{}, mockBilling, geocoding.NewClient(), nil, nil)

	owner := testutil.CreateTestDoctor(t, db, "doc_clinic_owner_autojoin", "password123")
	clinic := models.Clinic{Name: "Clínica Autojoin", OwnerDoctorID: owner.ID, SubscriptionStatus: "active", StripeSubscriptionID: "sub_clinic_autojoin"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&owner).Updates(map[string]any{"clinic_id": clinic.ID, "is_clinic_owner": true}).Error)

	invite := models.ClinicEmailInvite{
		ClinicID: clinic.ID, Email: "nuevo_colega@correo.local", InvitedByDoctorID: owner.ID,
		ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour),
	}
	require.NoError(t, db.Create(&invite).Error)

	newcomer := testutil.CreateTestDoctor(t, db, "doc_newcomer_autojoin", "password123")
	require.NoError(t, db.Model(&newcomer).Updates(map[string]any{
		"email":                  "nuevo_colega@correo.local",
		"cedula_validated":       "CAPTURADA",
		"subscription_status":    "active",
		"stripe_subscription_id": "sub_newcomer_autojoin",
	}).Error)

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_autojoin", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	docIDStr := strconv.FormatUint(uint64(newcomer.ID), 10)
	w := doRequest(t, router, http.MethodPut, "/api/admin/doctors/"+docIDStr+"/approve", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var reloaded struct {
		CedulaValidated      string
		ClinicID             *uint
		IsClinicOwner        bool
		SubscriptionStatus   string
		StripeSubscriptionID string
	}
	require.NoError(t, db.Table("doctors").
		Select("cedula_validated, clinic_id, is_clinic_owner, subscription_status, stripe_subscription_id").
		Where("id = ?", newcomer.ID).Scan(&reloaded).Error)
	assert.Equal(t, "VALIDADA", reloaded.CedulaValidated)
	require.NotNil(t, reloaded.ClinicID)
	assert.Equal(t, clinic.ID, *reloaded.ClinicID)
	assert.False(t, reloaded.IsClinicOwner)
	assert.Equal(t, "active", reloaded.SubscriptionStatus)
	assert.Empty(t, reloaded.StripeSubscriptionID)
	assert.True(t, mockBilling.wasCanceled("sub_newcomer_autojoin"))

	var reloadedInvite models.ClinicEmailInvite
	require.NoError(t, db.First(&reloadedInvite, invite.ID).Error)
	require.NotNil(t, reloadedInvite.ConsumedAt)
}

// TestSuperAdmin_RejectSendsDoctorBackToPendiente confirma que rechazar
// regresa al doctor a PENDIENTE (puede volver a subir su documentación) en
// vez de dejarlo bloqueado sin salida.
func TestSuperAdmin_RejectSendsDoctorBackToPendiente(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_rejected", "password123")
	doc.CedulaValidated = "CAPTURADA"
	require.NoError(t, db.Save(&doc).Error)

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_reject", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	docIDStr := strconv.FormatUint(uint64(doc.ID), 10)
	w := doRequest(t, router, http.MethodPut, "/api/admin/doctors/"+docIDStr+"/reject", adminToken, map[string]any{
		"reason": "La foto del INE está ilegible",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var reloaded struct{ CedulaValidated string }
	require.NoError(t, db.Table("doctors").Select("cedula_validated").Where("id = ?", doc.ID).Scan(&reloaded).Error)
	assert.Equal(t, "PENDIENTE", reloaded.CedulaValidated)
}

// TestSuperAdmin_CannotApproveDoctorNotPendingReview evita "aprobar" a un
// doctor que ni siquiera terminó su onboarding (todavía en PENDIENTE) o que
// ya estaba VALIDADA.
func TestSuperAdmin_CannotApproveDoctorNotPendingReview(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_still_pendiente", "password123")
	// CreateTestDoctor no toca CedulaValidated: arranca en "PENDIENTE" (default de la columna).

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_guard", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	docIDStr := strconv.FormatUint(uint64(doc.ID), 10)
	w := doRequest(t, router, http.MethodPut, "/api/admin/doctors/"+docIDStr+"/approve", adminToken, nil)
	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestGrantFreeAccess_UnlocksCanceledDoctor cubre el caso central: un
// doctor con la suscripción vencida (bloqueado por
// RequireActiveSubscription) recupera acceso completo en cuanto el
// superadmin le da acceso gratuito hasta una fecha futura.
func TestGrantFreeAccess_UnlocksCanceledDoctor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, nil, billing.Config{}, mockBilling, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_grant_free", "password123")
	require.NoError(t, db.Model(&doc).Update("subscription_status", "canceled").Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	// Bloqueado antes del acceso gratuito.
	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusPaymentRequired, w.Code)

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_grant_free", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	untilDate := time.Now().UTC().AddDate(0, 0, 30)
	docIDStr := strconv.FormatUint(uint64(doc.ID), 10)
	w = doRequest(t, router, http.MethodPut, "/api/admin/doctors/"+docIDStr+"/grant-free-access", adminToken, map[string]any{
		"until": untilDate.Format("2006-01-02"),
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var reloaded struct {
		SubscriptionStatus string
		TrialEndsAt        *time.Time
	}
	require.NoError(t, db.Table("doctors").Select("subscription_status, trial_ends_at").Where("id = ?", doc.ID).Scan(&reloaded).Error)
	assert.Equal(t, "trialing", reloaded.SubscriptionStatus)
	require.NotNil(t, reloaded.TrialEndsAt)
	// El handler fija las 23:59:59 de ese día (ver GrantFreeAccess), no la
	// misma hora del día en que se llamó.
	expectedUntil := time.Date(untilDate.Year(), untilDate.Month(), untilDate.Day(), 23, 59, 59, 0, time.UTC)
	assert.WithinDuration(t, expectedUntil, *reloaded.TrialEndsAt, 2*time.Minute)

	// Ya no está bloqueado.
	w = doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestGrantFreeAccess_CancelsExistingStripeSubscription confirma que a un
// doctor que YA pagaba de verdad no se le sigue cobrando mientras tiene
// acceso gratuito otorgado a mano.
func TestGrantFreeAccess_CancelsExistingStripeSubscription(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, nil, billing.Config{}, mockBilling, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_grant_free_active", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"subscription_status":    "active",
		"stripe_subscription_id": "sub_test_grant_free",
	}).Error)

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_grant_free_active", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	until := time.Now().UTC().AddDate(0, 1, 0).Format("2006-01-02")
	docIDStr := strconv.FormatUint(uint64(doc.ID), 10)
	w := doRequest(t, router, http.MethodPut, "/api/admin/doctors/"+docIDStr+"/grant-free-access", adminToken, map[string]any{
		"until": until,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.True(t, mockBilling.wasCanceled("sub_test_grant_free"))

	var reloaded struct {
		SubscriptionStatus   string
		StripeSubscriptionID string
	}
	require.NoError(t, db.Table("doctors").Select("subscription_status, stripe_subscription_id").Where("id = ?", doc.ID).Scan(&reloaded).Error)
	assert.Equal(t, "trialing", reloaded.SubscriptionStatus)
	assert.Empty(t, reloaded.StripeSubscriptionID)
}

// TestGrantFreeAccess_RejectsClinicMemberAndPastDate cubre las dos
// validaciones: no aplica a un doctor de clínica (su acceso depende de la
// clínica completa), y la fecha tiene que ser futura.
func TestGrantFreeAccess_RejectsClinicMemberAndPastDate(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_grant_free_clinic", "password123")
	clinic := models.Clinic{Name: "Clínica de prueba", OwnerDoctorID: doc.ID}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&doc).Update("clinic_id", clinic.ID).Error)

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_grant_free_clinic", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	docIDStr := strconv.FormatUint(uint64(doc.ID), 10)
	futureDate := time.Now().UTC().AddDate(0, 0, 10).Format("2006-01-02")
	w := doRequest(t, router, http.MethodPut, "/api/admin/doctors/"+docIDStr+"/grant-free-access", adminToken, map[string]any{
		"until": futureDate,
	})
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	doc2 := testutil.CreateTestDoctor(t, db, "doc_grant_free_past", "password123")
	pastDate := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	doc2IDStr := strconv.FormatUint(uint64(doc2.ID), 10)
	w = doRequest(t, router, http.MethodPut, "/api/admin/doctors/"+doc2IDStr+"/grant-free-access", adminToken, map[string]any{
		"until": pastDate,
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// createAdminTestAppointment crea una cita de prueba con fecha DENTRO del
// mes calendario actual (para que la agarren las estadísticas mensuales del
// panel), con el status y source que se le pida.
func createAdminTestAppointment(t *testing.T, db *gorm.DB, doctorID uint, status, source string) {
	t.Helper()
	patient := models.Patient{FirstName: "Paciente", LastName: "Admin", Phone: "5550000000"}
	require.NoError(t, db.Create(&patient).Error)

	appt := models.Appointment{
		PatientID:           patient.ID,
		DoctorID:            doctorID,
		AppointmentDateTime: time.Now().UTC(),
		Status:              status,
		Source:              source,
	}
	require.NoError(t, db.Create(&appt).Error)
}

// TestListAllDoctors_ReturnsAllWithMonthlyStatsAndSubscription confirma que
// el panel ve TODOS los doctores (no solo los pendientes de cédula), con su
// estatus de suscripción y sus citas del mes correctamente separadas por
// desenlace (completada/no-show/cancelada) y por canal de origen
// (agendada por el doctor vs. solicitada desde el directorio público).
func TestListAllDoctors_ReturnsAllWithMonthlyStatsAndSubscription(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_admin_list", "password123")
	require.NoError(t, db.Model(&doc).Update("subscription_status", "active").Error)

	createAdminTestAppointment(t, db, doc.ID, "COMPLETED", "DOCTOR")
	createAdminTestAppointment(t, db, doc.ID, "COMPLETED", "PUBLIC")
	createAdminTestAppointment(t, db, doc.ID, "NOSHOW", "DOCTOR")
	createAdminTestAppointment(t, db, doc.ID, "CANCELLED", "PUBLIC")

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_doctors_list", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	w := doRequest(t, router, http.MethodGet, "/api/admin/doctors", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var doctors []map[string]any
	decodeJSONList(t, w, &doctors)
	require.Len(t, doctors, 1)

	got := doctors[0]
	assert.Equal(t, "active", got["subscriptionStatus"])
	assert.Equal(t, false, got["isClinicMember"])

	stats, ok := got["monthlyStats"].(map[string]any)
	require.True(t, ok, "monthlyStats debe venir como objeto")
	assert.Equal(t, float64(4), stats["total"])
	assert.Equal(t, float64(2), stats["completed"])
	assert.Equal(t, float64(1), stats["noShow"])
	assert.Equal(t, float64(1), stats["cancelled"])
	assert.Equal(t, float64(2), stats["bookedByDoctor"])
	assert.Equal(t, float64(2), stats["bookedPublic"])
}

// TestListAllDoctors_ShowsCurrentPeriodEndForActiveSubscription confirma
// que un doctor con suscripción individual activa de verdad muestra la
// fecha de renovación (consultada en vivo a Stripe, ver
// GetSubscriptionPeriodEnd) — para que el admin sepa hasta cuándo tiene
// acceso pagado, igual que ya se ve la fecha de un acceso gratuito
// otorgado a mano (TrialEndsAt).
func TestListAllDoctors_ShowsCurrentPeriodEndForActiveSubscription(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	periodEnd := time.Now().UTC().AddDate(0, 0, 25)
	mockBilling.periodEnd = periodEnd
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, nil, billing.Config{}, mockBilling, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_admin_period_end", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"subscription_status":    "active",
		"stripe_subscription_id": "sub_admin_period_end",
	}).Error)

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_period_end", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	w := doRequest(t, router, http.MethodGet, "/api/admin/doctors", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var doctors []map[string]any
	decodeJSONList(t, w, &doctors)
	require.Len(t, doctors, 1)
	require.NotNil(t, doctors[0]["currentPeriodEnd"])
	parsed, err := time.Parse(time.RFC3339, doctors[0]["currentPeriodEnd"].(string))
	require.NoError(t, err)
	assert.WithinDuration(t, periodEnd, parsed, time.Second)
}

// TestListAllDoctors_ClinicMemberUsesClinicSubscriptionStatus confirma que
// un doctor perteneciente a una clínica muestra el estatus de suscripción
// de LA CLÍNICA, no el suyo propio (que deja de ser relevante en cuanto se
// une, ver billing.RequireActiveSubscription).
func TestListAllDoctors_ClinicMemberUsesClinicSubscriptionStatus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_admin_clinic_owner", "password123")
	member := testutil.CreateTestDoctor(t, db, "doc_admin_clinic_member", "password123")

	clinic := models.Clinic{Name: "Clínica de Prueba", OwnerDoctorID: owner.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&member).Updates(map[string]any{"clinic_id": clinic.ID}).Error)

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_clinic_status", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	w := doRequest(t, router, http.MethodGet, "/api/admin/doctors", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var doctors []map[string]any
	decodeJSONList(t, w, &doctors)
	require.Len(t, doctors, 2)

	var memberEntry map[string]any
	for _, d := range doctors {
		if d["id"] == float64(member.ID) {
			memberEntry = d
		}
	}
	require.NotNil(t, memberEntry, "el doctor miembro de la clínica debe aparecer en la lista")
	assert.Equal(t, "active", memberEntry["subscriptionStatus"])
	assert.Equal(t, true, memberEntry["isClinicMember"])
	assert.Equal(t, "Clínica de Prueba", memberEntry["clinicName"])
}

// TestGetAdminOverview_AggregatesAcrossAllDoctors confirma que el resumen
// de la plataforma suma correctamente las citas del mes de TODOS los
// doctores, no solo de uno.
func TestGetAdminOverview_AggregatesAcrossAllDoctors(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doc_admin_overview_a", "password123")
	docB := testutil.CreateTestDoctor(t, db, "doc_admin_overview_b", "password123")

	createAdminTestAppointment(t, db, docA.ID, "COMPLETED", "DOCTOR")
	createAdminTestAppointment(t, db, docB.ID, "COMPLETED", "PUBLIC")
	createAdminTestAppointment(t, db, docB.ID, "NOSHOW", "PUBLIC")

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_overview", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	w := doRequest(t, router, http.MethodGet, "/api/admin/overview", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	resp := decodeJSON(t, w)
	assert.Equal(t, float64(2), resp["totalDoctors"])

	stats, ok := resp["monthlyStats"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(3), stats["total"])
	assert.Equal(t, float64(2), stats["completed"])
	assert.Equal(t, float64(1), stats["noShow"])
	assert.Equal(t, float64(1), stats["bookedByDoctor"])
	assert.Equal(t, float64(2), stats["bookedPublic"])
}

// TestCreateAppointment_TagsSourceAsDoctor y
// TestPublicAppointment_TagsSourceAsPublic confirman que cada punto de
// creación de citas marca correctamente el origen (ver
// models.Appointment.Source) — la base de todo lo que agrega este panel.
func TestCreateAppointment_TagsSourceAsDoctor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_source_direct", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		"service":             "Consulta general",
		"patientFirstName":    "Directo",
		"patientLastName":     "Prueba",
		"patientPhone":        "5551230000",
		"registrationStatus":  "REGISTERED",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var appt models.Appointment
	require.NoError(t, db.Where("doctor_id = ?", doc.ID).First(&appt).Error)
	assert.Equal(t, "DOCTOR", appt.Source)
}

func TestPublicAppointment_TagsSourceAsPublic(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_source_public", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-source-public-1"}).Error)

	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Público",
		"patientLastName":     "Prueba",
		"patientPhone":        "5559998888",
		"patientEmail":        "publico.source@test.local",
		"dataConsent":         true,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var appt models.Appointment
	require.NoError(t, db.Where("doctor_id = ?", doc.ID).First(&appt).Error)
	assert.Equal(t, "PUBLIC", appt.Source)
}
