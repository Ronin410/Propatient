package server_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetTodaySummary_LateEveningAppointmentCountsAsToday cubre el bug
// reportado: una cita agendada tarde en la noche, hora local del cliente
// (ej. 11:30pm en UTC-7), se guarda en UTC como la madrugada del día
// SIGUIENTE. El resumen de "hoy" del dashboard debe seguir contándola como
// hoy si "hoy" es hoy para el cliente que la agendó — antes se comparaba
// con DATE(appointment_date_time), que usa la zona horaria de la sesión de
// Postgres (UTC) en vez de la del cliente, y la cita desaparecía de la
// lista/conteo de "hoy".
func TestGetTodaySummary_LateEveningAppointmentCountsAsToday(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_tz_evening", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	// UTC-7 fijo (Culiacán/Mazatlán) — fechas fijas en el futuro, sin
	// relación con la hora real de la corrida, para que el test no sea
	// sensible a cuándo se ejecute (a diferencia de TestConsultorioStats).
	utcMinus7 := time.FixedZone("UTC-7", -7*3600)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Nocturno", "email": "nocturno@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	// Cita a las 11:30pm hora local (UTC-7) = 06:30 UTC del día siguiente.
	apptLocal := time.Date(2030, 6, 20, 23, 30, 0, 0, utcMinus7)
	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": apptLocal.Format(time.RFC3339),
		"service":             "Consulta nocturna",
		"patientId":           patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// El dashboard se consulta esa misma tarde/noche, hora local — clientTime
	// sigue siendo "20 de junio" para el cliente, aunque en UTC ya casi sea
	// "21 de junio".
	clientNow := time.Date(2030, 6, 20, 20, 0, 0, 0, utcMinus7)
	clientTimeParam := url.QueryEscape(clientNow.Format(time.RFC3339))

	w = doRequest(t, router, http.MethodGet, "/api/dashboard/summary?clientTime="+clientTimeParam, token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	summary := decodeJSON(t, w)

	assert.Equal(t, float64(1), summary["todayCount"],
		"una cita de las 11:30pm hora local debe contar como 'hoy' aunque en UTC caiga en el día siguiente")

	todayAppointments, ok := summary["todayAppointments"].([]any)
	require.True(t, ok)
	assert.Len(t, todayAppointments, 1)
}

// TestGetTodaySummary_NextDayMorningNotCountedAsToday confirma el otro
// lado: una cita de la madrugada del día SIGUIENTE (hora local) no debe
// colarse en el resumen de "hoy", ni siquiera si su marca UTC cae en el
// mismo día UTC que "hoy".
func TestGetTodaySummary_NextDayMorningNotCountedAsToday(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_tz_nextday", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	utcMinus7 := time.FixedZone("UTC-7", -7*3600)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Madrugada", "email": "madrugada@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	// Cita a las 2:00am hora local del día SIGUIENTE.
	apptLocal := time.Date(2030, 6, 21, 2, 0, 0, 0, utcMinus7)
	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": apptLocal.Format(time.RFC3339),
		"service":             "Consulta madrugada",
		"patientId":           patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	clientNow := time.Date(2030, 6, 20, 20, 0, 0, 0, utcMinus7)
	clientTimeParam := url.QueryEscape(clientNow.Format(time.RFC3339))

	w = doRequest(t, router, http.MethodGet, "/api/dashboard/summary?clientTime="+clientTimeParam, token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	summary := decodeJSON(t, w)

	assert.Equal(t, float64(0), summary["todayCount"],
		"una cita de las 2am hora local del día siguiente no debe contar como 'hoy'")
}
