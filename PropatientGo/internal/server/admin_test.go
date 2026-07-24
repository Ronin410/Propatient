package server_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

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
