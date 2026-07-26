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

func TestGetAppointments_StatusFilter(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doctor_status_filter", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Filtro", "email": "filtro@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-07-14T10:00:00Z",
		"service":             "Consulta",
		"patientId":           patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	// Cancelamos la cita para tener una PENDING (ninguna) y una CANCELLED.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID)+"/cancel", token, nil)
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=CANCELLED", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var cancelled []map[string]any
	decodeJSONList(t, w, &cancelled)
	assert.Len(t, cancelled, 1)

	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=PENDING", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var pending []map[string]any
	decodeJSONList(t, w, &pending)
	assert.Empty(t, pending)
}

func TestGetAppointments_InvalidStatusRejected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doctor_bad_status", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/appointments?status=NOT_A_REAL_STATUS", token, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAppointments_MalformedDateRejected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doctor_bad_date", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	// Antes, una fecha mal formada se ignoraba en silencio y devolvía 200
	// con la lista sin filtrar. Ahora debe rechazarse con 400.
	w := doRequest(t, router, http.MethodGet, "/api/appointments?start=not-a-date&end=2026-07-14", token, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAppointments_OnlyOneDateRejected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doctor_one_date", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/appointments?start=2026-07-14", token, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
