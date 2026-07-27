package server_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsultorioStats(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doctor_stats", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Stats", "email": "stats@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	now := time.Now().UTC()
	// Antes esto usaba un día fijo (15) del mes actual, lo que rompía el
	// test en cuanto se corría después del día 15: esa fecha quedaba en el
	// pasado y dejaba de contar como "upcomingAppointments" (que exige
	// appointment_date_time >= now, ver GetConsultorioStats). Usar "dentro
	// de un par de horas" garantiza que siempre esté en el futuro y, además,
	// siempre dentro del mes calendario de "now" (nunca cruza a fin de mes).
	thisMonth := now.Add(2 * time.Hour).Format(time.RFC3339)

	// Una cita agendada este mes (se queda PENDING).
	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": thisMonth, "service": "Consulta 1", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	// Una cita cancelada este mes.
	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": thisMonth, "service": "Consulta 2", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	appt2 := int(decodeJSON(t, w)["id"].(float64))
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(appt2)+"/cancel", token, nil)
	require.Equal(t, http.StatusOK, w.Code)

	// Una cita pendiente en los próximos 30 días. A propósito NO se asume
	// que caiga en el mismo mes calendario que "now" — cerca de fin de mes
	// (día 27+ en un mes de 31 días) +5 días cae en el mes siguiente, y
	// GetConsultorioStats separa "appointmentsThisMonth" (por mes
	// calendario) de "upcomingAppointments" (por ventana de 30 días, sin
	// importar el mes). El valor esperado se calcula según corresponda, en
	// vez de asumir siempre el mismo mes (eso rompía el test cerca de fin
	// de mes).
	upcomingTime := now.Add(5 * 24 * time.Hour)
	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": upcomingTime.Format(time.RFC3339), "service": "Consulta futura", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/dashboard/stats", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	stats := decodeJSON(t, w)

	// "Consulta 1" y "Consulta 2" siempre cuentan (mismo mes que "now" por
	// construcción); "Consulta futura" solo si +5 días no cruzó de mes.
	expectedAppointmentsThisMonth := 2
	if upcomingTime.Year() == now.Year() && upcomingTime.Month() == now.Month() {
		expectedAppointmentsThisMonth = 3
	}

	assert.Equal(t, float64(1), stats["totalPatients"])
	assert.Equal(t, float64(expectedAppointmentsThisMonth), stats["appointmentsThisMonth"])
	assert.Equal(t, float64(1), stats["cancelledThisMonth"])
	assert.Equal(t, float64(0), stats["completedThisMonth"])
	// upcomingAppointments = 2: tanto "Consulta 1" (sigue PENDING) como
	// "Consulta futura" (+5 días) caen dentro de "próximos 30 días y
	// PENDING", sin importar el mes calendario. Solo "Consulta 2" queda
	// fuera por estar CANCELLED.
	assert.Equal(t, float64(2), stats["upcomingAppointments"])
}

func TestConsultorioStats_IsolatedPerDoctor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_stats", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_stats", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Paciente", "lastName": "DeA", "email": "pacientea_stats@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/dashboard/stats", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code)
	stats := decodeJSON(t, w)
	assert.Equal(t, float64(0), stats["totalPatients"], "las métricas de Doctor B no deben incluir datos de Doctor A")
}
