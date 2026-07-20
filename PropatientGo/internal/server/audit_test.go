package server_test

import (
	"fmt"
	"net/http"
	"testing"

	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestAppointment agenda una cita nueva para el paciente indicado y
// regresa su ID — helper compartido por los tests de auditoría/historial.
func createTestAppointment(t *testing.T, router *gin.Engine, token string, patientID int) int {
	t.Helper()
	w := doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z",
		"service":             "Consulta general",
		"patientId":           patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	return int(decodeJSON(t, w)["id"].(float64))
}

// TestAudit_CreatePatient_LogsEntry confirma que dar de alta a un paciente
// deja una entrada en la bitácora, con el actor y la IP correctos.
func TestAudit_CreatePatient_LogsEntry(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_audit_create_patient", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Auditoría", "email": "audit_patient1@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/patients/%d/audit-log", patientID), token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var entries []map[string]any
	decodeJSONList(t, w, &entries)
	require.Len(t, entries, 1)
	assert.Equal(t, "created", entries[0]["action"])
	assert.Equal(t, "patient", entries[0]["entityType"])
	assert.Equal(t, "MEDICO", entries[0]["actorRole"])
	assert.Equal(t, doc.FullName, entries[0]["actorName"])
	assert.NotEmpty(t, entries[0]["ipAddress"])
}

// TestAudit_ViewMedicalHistory_LogsViewedEntry confirma que consultar el
// expediente clínico completo también queda registrado, no solo los cambios.
func TestAudit_ViewMedicalHistory_LogsViewedEntry(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_audit_view", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Vista", "email": "audit_patient2@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/patients/%d/history", patientID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/patients/%d/audit-log", patientID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var entries []map[string]any
	decodeJSONList(t, w, &entries)

	found := false
	for _, e := range entries {
		if e["action"] == "viewed" && e["entityType"] == "medical_history" {
			found = true
		}
	}
	assert.True(t, found, "debe existir una entrada 'viewed' de medical_history: %+v", entries)
}

// TestAudit_StaffCannotAccessAuditLog confirma que la bitácora es solo del
// doctor, igual que el resto del contenido clínico.
func TestAudit_StaffCannotAccessAuditLog(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_audit_staff", "password123")
	staff := testutil.CreateTestStaff(t, db, doc.ID, "staff_audit@test.local", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodGet, "/api/patients/1/audit-log", staffToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestNoteHistory_UpdateAppointment_PreservesClinicalContent es el test
// central: editar el diagnóstico/tratamiento/notas de una cita no debe
// perder el valor anterior — debe quedar preservado en
// AppointmentNoteHistory, y la segunda edición debe preservar el valor de
// la primera (no el original vacío).
func TestNoteHistory_UpdateAppointment_PreservesClinicalContent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_notehist_1", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "NotaUno", "email": "notehist_patient1@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	apptID := createTestAppointment(t, router, token, patientID)

	// Primera edición: diagnóstico vacío -> "Gripe común".
	w = doRequest(t, router, http.MethodPut, fmt.Sprintf("/api/appointments/%d", apptID), token, map[string]any{
		"diagnosis": "Gripe común",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Segunda edición: "Gripe común" -> "Gripe común, se corrige a faringitis".
	w = doRequest(t, router, http.MethodPut, fmt.Sprintf("/api/appointments/%d", apptID), token, map[string]any{
		"diagnosis": "Faringitis",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Faringitis", decodeJSON(t, w)["diagnosis"])

	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/appointments/%d/note-history", apptID), token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var history []map[string]any
	decodeJSONList(t, w, &history)
	require.Len(t, history, 2, "debe haber una versión preservada por cada edición que cambió contenido clínico")

	// Orden DESC: la más reciente primero — preserva "Gripe común" (el
	// valor justo antes de corregirlo a "Faringitis"), NUNCA se perdió.
	assert.Equal(t, "Gripe común", history[0]["previousDiagnosis"])
	// La primera edición preservó el estado original: vacío.
	assert.Equal(t, "", history[1]["previousDiagnosis"])
	assert.Equal(t, "MEDICO", history[0]["changedByRole"])
	assert.Equal(t, doc.FullName, history[0]["changedByName"])
}

// TestNoteHistory_NonClinicalUpdate_DoesNotCreateHistoryEntry confirma que
// un PUT que no toca contenido clínico (ej. solo reprogramar) no ensucia
// el historial con versiones idénticas.
func TestNoteHistory_NonClinicalUpdate_DoesNotCreateHistoryEntry(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_notehist_2", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "NotaDos", "email": "notehist_patient2@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	apptID := createTestAppointment(t, router, token, patientID)

	w = doRequest(t, router, http.MethodPut, fmt.Sprintf("/api/appointments/%d", apptID), token, map[string]any{
		"reason": "Consulta general (motivo actualizado)",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/appointments/%d/note-history", apptID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var history []map[string]any
	decodeJSONList(t, w, &history)
	assert.Empty(t, history, "un cambio que no toca contenido clínico no debe generar una versión")
}

// TestNoteHistory_ScopedToDoctor confirma que un doctor no puede ver el
// historial de notas de una cita ajena (IDOR).
func TestNoteHistory_ScopedToDoctor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doc_notehist_a", "password123")
	docB := testutil.CreateTestDoctor(t, db, "doc_notehist_b", "password123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Paciente", "lastName": "DeA3", "email": "notehist_patient3@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))
	apptID := createTestAppointment(t, router, tokenA, patientID)

	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/appointments/%d/note-history", apptID), tokenB, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
