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

// TestCreatePatient_DuplicateEmail_ReturnsConflict cubre la deduplicación de
// pacientes entre doctores: un mismo paciente no debe quedar registrado dos
// veces solo porque lo atendió otro doctor del sistema.
func TestCreatePatient_DuplicateEmail_ReturnsConflict(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_link", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_link", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Original", "lastName": "Paciente", "email": "compartido@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	originalID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/patients", tokenB, map[string]any{
		"firstName": "Otro", "lastName": "Nombre", "email": "compartido@test.local",
	})
	require.Equal(t, http.StatusConflict, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "patient_exists", body["error"])
	patientInfo := body["patient"].(map[string]any)
	assert.Equal(t, float64(originalID), patientInfo["id"])
}

// TestCreatePatient_NoEmail_DoesNotCollide es la regresión del bug real: con
// email marcado como "unique" a nivel de columna, dos pacientes sin correo
// (campo opcional) de doctores distintos colisionaban en el mismo valor "".
func TestCreatePatient_NoEmail_DoesNotCollide(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_noemail", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_noemail", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "SinCorreo", "lastName": "Uno", "email": "",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/patients", tokenB, map[string]any{
		"firstName": "SinCorreo", "lastName": "Dos", "email": "",
	})
	require.Equal(t, http.StatusCreated, w.Code, "dos pacientes sin correo de doctores distintos deben poder crearse")
}

// TestLinkExistingPatient_SharesHistoryNotAppointments verifica el requisito
// central: al vincularse a un paciente ya existente, el doctor ve sus
// antecedentes pero NUNCA las citas que ese paciente tuvo con otro doctor.
func TestLinkExistingPatient_SharesHistoryNotAppointments(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_share", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_share", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Compartido", "lastName": "Paciente", "email": "compartido2@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPut, "/api/patients/"+strconv.Itoa(patientID)+"/medical-history", tokenA, map[string]any{
		"allergies": "Penicilina",
	})
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/appointments", tokenA, map[string]any{
		"appointmentDateTime": "2026-07-14T10:00:00Z", "service": "Consulta con A", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Doctor B, sin haber tenido nunca a este paciente, se vincula.
	w = doRequest(t, router, http.MethodPost, "/api/patients/"+strconv.Itoa(patientID)+"/link", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code)
	linked := decodeJSON(t, w)
	history := linked["medicalHistory"].(map[string]any)
	assert.Equal(t, "Penicilina", history["allergies"], "debe ver los antecedentes ya registrados por el otro doctor")

	// El historial de B para este paciente no debe traer la cita de A.
	w = doRequest(t, router, http.MethodGet, "/api/patients/"+strconv.Itoa(patientID)+"/history", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code)
	historyForB := decodeJSON(t, w)
	appointments, _ := historyForB["appointments"].([]any)
	assert.Empty(t, appointments, "el doctor B no debe ver las citas que el paciente tuvo con el doctor A")

	// B agenda su propia cita: esa sí debe aparecer en SU historial.
	w = doRequest(t, router, http.MethodPost, "/api/appointments", tokenB, map[string]any{
		"appointmentDateTime": "2026-07-15T10:00:00Z", "service": "Consulta con B", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/patients/"+strconv.Itoa(patientID)+"/history", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code)
	historyForB = decodeJSON(t, w)
	appointments, _ = historyForB["appointments"].([]any)
	assert.Len(t, appointments, 1, "debe ver únicamente su propia cita con el paciente compartido")
}
