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

// TestReschedule_KeepsOtherFields cubre la feature de "reprogramar cita" del
// frontend: un PUT parcial que solo manda appointmentDateTime no debe borrar
// el resto de los campos de la cita (reason, status, etc.).
func TestReschedule_KeepsOtherFields(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doctor_reschedule", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Reprog", "email": "reprog@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-07-14T10:00:00Z",
		"service":             "Consulta original",
		"patientId":           patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	// Reprogramamos: solo mandamos la nueva fecha, como hace el frontend.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID), token, map[string]any{
		"appointmentDateTime": "2026-07-20T15:30:00Z",
	})
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(apptID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	appt := decodeJSON(t, w)

	assert.Equal(t, "2026-07-20T15:30:00Z", appt["appointmentDateTime"])
	assert.Equal(t, "Consulta original", appt["reason"], "el motivo de consulta no debería perderse al reprogramar")
	assert.Equal(t, "PENDING", appt["status"], "el estado no debería cambiar solo por reprogramar")
}
