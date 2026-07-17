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

// weekdayOnly es un horario con lunes a viernes habilitados (6:00-19:00,
// descanso 14:00-15:00) y fin de semana deshabilitado — usado en varios
// tests de este archivo. 2026-07-20 es lunes, 2026-07-25 es sábado (ver
// comentarios en cada test).
func weekdayOnlySchedule() map[string]any {
	weekday := map[string]any{
		"enabled": true, "start": "06:00", "end": "19:00",
		"breaks": []map[string]any{{"start": "14:00", "end": "15:00"}},
	}
	weekend := map[string]any{"enabled": false, "start": "", "end": "", "breaks": []map[string]any{}}
	return map[string]any{
		"days": map[string]any{
			"sunday": weekend, "monday": weekday, "tuesday": weekday, "wednesday": weekday,
			"thursday": weekday, "friday": weekday, "saturday": weekend,
		},
	}
}

// TestDoctorSchedule_GetDefaultsToUnconfigured confirma que, sin haber
// guardado nunca un horario, GET devuelve "configured: false" (no un
// error) y que crear una cita a cualquier hora sigue funcionando sin
// restricción — comportamiento de siempre para quien no usa esta función.
func TestDoctorSchedule_GetDefaultsToUnconfigured(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_sched_default", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/schedule", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, false, body["configured"])

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-07-25T03:00:00Z", // sábado de madrugada, sin restricción
		"service":             "Consulta libre",
		"patientFirstName":    "Paciente", "patientLastName": "Libre",
	})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// TestDoctorSchedule_SaveAndRetrieve confirma el guardado (upsert) y que
// una segunda llamada a GET devuelve exactamente lo guardado.
func TestDoctorSchedule_SaveAndRetrieve(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_sched_save", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPut, "/api/doctor/schedule", token, weekdayOnlySchedule())
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, true, body["configured"])

	w = doRequest(t, router, http.MethodGet, "/api/doctor/schedule", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body = decodeJSON(t, w)
	assert.Equal(t, true, body["configured"])
	days, ok := body["days"].(map[string]any)
	require.True(t, ok)
	monday, ok := days["monday"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "06:00", monday["start"])
	assert.Equal(t, "19:00", monday["end"])
}

// TestDoctorSchedule_SaveRejectsInvalidRanges cubre la validación al
// guardar: inicio después del fin, y un descanso fuera de la ventana
// laboral del día.
func TestDoctorSchedule_SaveRejectsInvalidRanges(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_sched_invalid", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPut, "/api/doctor/schedule", token, map[string]any{
		"days": map[string]any{
			"monday": map[string]any{"enabled": true, "start": "20:00", "end": "08:00", "breaks": []map[string]any{}},
		},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doRequest(t, router, http.MethodPut, "/api/doctor/schedule", token, map[string]any{
		"days": map[string]any{
			"monday": map[string]any{
				"enabled": true, "start": "08:00", "end": "18:00",
				"breaks": []map[string]any{{"start": "19:00", "end": "20:00"}},
			},
		},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDoctorSchedule_StaffCanViewAndEdit confirma que el personal (no solo
// el doctor) puede consultar y guardar el horario — a propósito distinto
// del resto de /doctor/*, que es exclusivo del doctor.
func TestDoctorSchedule_StaffCanViewAndEdit(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_sched_staff", "password123")
	staff := testutil.CreateTestStaff(t, db, doc.ID, "personal_sched@test.local", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/schedule", staffToken, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodPut, "/api/doctor/schedule", staffToken, weekdayOnlySchedule())
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Pero el resto de /doctor/* sigue bloqueado para personal.
	w = doRequest(t, router, http.MethodGet, "/api/doctor/me", staffToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestCreateAppointment_BlockedOutsideWorkingHours cubre los tres motivos
// de bloqueo (día deshabilitado, fuera de la ventana laboral, dentro de un
// descanso) y confirma que un horario válido sí se acepta — usando
// /api/appointments (agenda interna del doctor/personal).
func TestCreateAppointment_BlockedOutsideWorkingHours(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_sched_block", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPut, "/api/doctor/schedule", token, weekdayOnlySchedule())
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	cases := []struct {
		name       string
		datetime   string // UTC, America/Mazatlan es UTC-7 sin horario de verano
		wantStatus int
	}{
		{"lunes 10:00 local, dentro de horario", "2026-07-20T17:00:00Z", http.StatusCreated},
		{"lunes 14:30 local, dentro del descanso", "2026-07-20T21:30:00Z", http.StatusBadRequest},
		{"lunes 20:00 local, ya cerró", "2026-07-21T03:00:00Z", http.StatusBadRequest},
		{"sábado 10:00 local, día deshabilitado", "2026-07-25T17:00:00Z", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
				"appointmentDateTime": tc.datetime,
				"service":             "Consulta",
				"patientFirstName":    "Paciente", "patientLastName": "Prueba",
			})
			assert.Equal(t, tc.wantStatus, w.Code, w.Body.String())
		})
	}
}

// TestUpdateAppointment_RescheduleBlockedOutsideWorkingHours confirma que
// reprogramar (PUT /appointments/:id) respeta el mismo horario, y que un
// PUT que no cambia la fecha (ej. solo notas) no se ve afectado por él.
func TestUpdateAppointment_RescheduleBlockedOutsideWorkingHours(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_sched_reschedule", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPut, "/api/doctor/schedule", token, weekdayOnlySchedule())
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-07-20T17:00:00Z", // lunes 10:00 local, válido
		"service":             "Consulta", "patientFirstName": "Paciente", "patientLastName": "Reprograma",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	appointmentID := int(decodeJSON(t, w)["id"].(float64))

	// Reprogramar a un horario inválido (sábado): debe rechazar.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(appointmentID), token, map[string]any{
		"appointmentDateTime": "2026-07-25T17:00:00Z",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Reprogramar a un horario válido: debe aceptar.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(appointmentID), token, map[string]any{
		"appointmentDateTime": "2026-07-21T18:00:00Z", // martes 11:00 local
	})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Un PUT que no toca la fecha (ej. solo notas) no debe fallar por el
	// horario, aunque la fecha ya guardada quedara "vieja".
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(appointmentID), token, map[string]any{
		"appointmentDateTime": "2026-07-21T18:00:00Z",
		"notes":               "Nota de seguimiento",
	})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestPublicAppointment_BlockedOutsideWorkingHours confirma que el
// agendado público (sin cuenta) respeta el mismo horario que la agenda
// interna.
func TestPublicAppointment_BlockedOutsideWorkingHours(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_sched_public", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-horario-publico"}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPut, "/api/doctor/schedule", token, weekdayOnlySchedule())
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-07-25T17:00:00Z", // sábado, deshabilitado
		"patientFirstName":    "Paciente", "patientLastName": "Publico",
		"patientPhone": "5512345678", "patientEmail": "publico_bloqueado@test.local",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	w = doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-07-20T17:00:00Z", // lunes 10:00 local, válido
		"patientFirstName":    "Paciente", "patientLastName": "Publico",
		"patientPhone": "5512345678", "patientEmail": "publico_valido@test.local",
	})
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// TestGetPublicDoctorBySlug_IncludesSchedule confirma que el perfil
// público del doctor expone el horario configurado — el paciente debe
// poder verlo antes de intentar agendar.
func TestGetPublicDoctorBySlug_IncludesSchedule(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_sched_slug", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-slug-horario"}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPut, "/api/doctor/schedule", token, weekdayOnlySchedule())
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/public/doctors/dr-slug-horario", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	schedule, ok := body["schedule"].(map[string]any)
	require.True(t, ok, "el perfil público debe incluir el horario")
	monday, ok := schedule["monday"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "06:00", monday["start"])
}
