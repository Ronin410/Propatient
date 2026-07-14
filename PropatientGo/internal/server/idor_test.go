package server_test

import (
	"fmt"
	"net/http"
	"testing"

	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIDOR_DoctorCannotCreateAppointmentForAnothersPatient(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_appt", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_appt", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Paciente", "lastName": "DeA", "email": "pacientea_appt@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	// Doctor B intenta agendar una cita usando el ID del paciente de A.
	w = doRequest(t, router, http.MethodPost, "/api/appointments", tokenB, map[string]any{
		"appointmentDateTime": "2026-07-14T10:00:00Z",
		"service":             "Consulta",
		"patientId":           patientID,
	})
	assert.Equal(t, http.StatusNotFound, w.Code, "Doctor B no debería poder agendar citas con el paciente de Doctor A")
}

func TestIDOR_DoctorCannotSeeAnothersPatient(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_see", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_see", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Paciente", "lastName": "DeA2", "email": "pacientea_see@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	// Doctor B intenta ver directamente al paciente de A por ID.
	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/patients/%d", patientID), tokenB, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// El listado de pacientes de Doctor B no debe incluir al de A.
	w = doRequest(t, router, http.MethodGet, "/api/patients", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, float64(0), resp["total"])
}

func TestIDOR_DoctorCannotCancelAnothersAppointment(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_cancel", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_cancel", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Paciente", "lastName": "DeA3", "email": "pacientea_cancel@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", tokenA, map[string]any{
		"appointmentDateTime": "2026-07-14T10:00:00Z",
		"service":             "Consulta",
		"patientId":           patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	// Doctor B intenta cancelar la cita de Doctor A.
	w = doRequest(t, router, http.MethodPut, fmt.Sprintf("/api/appointments/%d/cancel", apptID), tokenB, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// La cita de A debe seguir PENDING.
	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/appointments/%d", apptID), tokenA, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "PENDING", decodeJSON(t, w)["status"])
}

func TestIDOR_DoctorCannotRemoveAnothersPatient(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_remove", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_remove", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Paciente", "lastName": "DeA4", "email": "pacientea_remove@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	// Doctor B intenta desvincular al paciente de A.
	w = doRequest(t, router, http.MethodDelete, fmt.Sprintf("/api/patients/%d", patientID), tokenB, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Doctor A debe seguir viendo a su paciente sin problema.
	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/patients/%d", patientID), tokenA, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIDOR_DoctorCannotUpdateAnothersDocument(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_doc", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_doc", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Paciente", "lastName": "DeA5", "email": "pacientea_doc@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", tokenA, map[string]any{
		"appointmentDateTime": "2026-07-14T10:00:00Z",
		"service":             "Consulta",
		"patientId":           patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	// Doctor B intenta renombrar un documento (inexistente aún) de la cita de A;
	// debe dar 404 por no pertenecerle, no un error distinto que revele su existencia.
	w = doRequest(t, router, http.MethodPut, fmt.Sprintf("/api/appointments/%d/documents/1", apptID), tokenB, map[string]any{
		"filename": "hackeado.pdf",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDoctorTemplate_IsolatedPerDoctor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_tmpl", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_tmpl", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/doctor/template", tokenA, map[string]any{
		"fields": []map[string]any{{"id": "a", "label": "Solo de A", "placeholder": "", "required": false}},
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Doctor B pide SU plantilla: no debe existir todavía (nunca la guardó),
	// y sobre todo no debe ver la de Doctor A (bug del doctorID hardcodeado).
	w = doRequest(t, router, http.MethodGet, "/api/doctor/template", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	fields, ok := resp["fields"].([]any)
	require.True(t, ok)
	assert.Empty(t, fields, "Doctor B no debería ver la plantilla de Doctor A")
}

func TestGetPatients_Pagination(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doctor_pagination", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	for i := 0; i < 3; i++ {
		w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
			"firstName": fmt.Sprintf("Paciente%d", i), "lastName": "Test", "email": fmt.Sprintf("pag%d@test.local", i),
		})
		require.Equal(t, http.StatusCreated, w.Code)
	}

	w := doRequest(t, router, http.MethodGet, "/api/patients?page=1&pageSize=2", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, float64(3), resp["total"])
	assert.Equal(t, float64(2), resp["totalPages"])
	assert.Len(t, resp["data"], 2)

	w = doRequest(t, router, http.MethodGet, "/api/patients?page=2&pageSize=2", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	resp = decodeJSON(t, w)
	assert.Len(t, resp["data"], 1)
}
