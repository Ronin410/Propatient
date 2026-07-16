package server_test

import (
	"net/http"
	"strconv"
	"testing"

	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
