package server_test

import (
	"errors"
	"net/http"
	"testing"

	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestExportMyData_ReturnsDoctorPatientsAppointmentsStaff(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_export", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)
	testutil.CreateTestStaff(t, db, doc.ID, "personal_export@test.local", "clave123456")

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Export", "email": "export@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/user/export-data", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)

	doctorData, ok := body["doctor"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "doc_export", doctorData["Username"])

	patients, ok := body["patients"].([]any)
	require.True(t, ok)
	require.Len(t, patients, 1)

	appointments, ok := body["appointments"].([]any)
	require.True(t, ok)
	require.Len(t, appointments, 1)

	staff, ok := body["staff"].([]any)
	require.True(t, ok)
	require.Len(t, staff, 1)
}

// TestExportMyData_StaffCannotAccess confirma que el personal nunca puede
// descargar el expediente completo del consultorio.
func TestExportMyData_StaffCannotAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_export_staff", "password123")
	staff := testutil.CreateTestStaff(t, db, doc.ID, "personal_no_export@test.local", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodGet, "/api/user/export-data", staffToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestDeleteMyAccount_SoftDeletesAndBlocksLogin confirma que borrar la
// cuenta la excluye de login (soft-delete) sin borrar en cascada al
// paciente vinculado.
func TestDeleteMyAccount_SoftDeletesAndBlocksLogin(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_delete_me", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Conservado", "email": "conservado@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := decodeJSON(t, w)["id"].(float64)

	w = doRequest(t, router, http.MethodDelete, "/api/user/account", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// El login (que usa First(), excluye soft-deleted) ya no debe funcionar.
	w = doRequest(t, router, http.MethodPost, "/api/auth/login", "", map[string]any{
		"username": "doc_delete_me", "password": "password123",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// El registro sigue en la tabla (soft-delete, no hard-delete).
	var count int64
	db.Unscoped().Model(&models.Doctor{}).Where("username = ?", "doc_delete_me").Count(&count)
	assert.Equal(t, int64(1), count)

	// El paciente vinculado se conserva (no se borra en cascada).
	var patient models.Patient
	err := db.First(&patient, uint(patientID)).Error
	require.False(t, errors.Is(err, gorm.ErrRecordNotFound), "el paciente no debe borrarse al eliminar la cuenta del doctor")
}

// TestDeleteMyAccount_BlockedWhileSubscriptionActive evita que alguien
// "elimine su cuenta" para dejar de pagar sin cancelar primero.
func TestDeleteMyAccount_BlockedWhileSubscriptionActive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_active_sub", "password123")
	require.NoError(t, db.Model(&doc).Update("subscription_status", "active").Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodDelete, "/api/user/account", token, nil)
	assert.Equal(t, http.StatusConflict, w.Code)
}
