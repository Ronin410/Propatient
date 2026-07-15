package server_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"propatient-api/internal/billing"
	"propatient-api/internal/geocoding"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/storage"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogleCalendarSync_SkippedWhenNotConnected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mock := newMockCalendarClient()
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, mock, testStorage, billing.Config{}, nil, geocoding.NewClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doctor_cal_off", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "SinCalendar", "email": "sincal@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	assert.Equal(t, 0, mock.createdCount(), "sin refresh token guardado, no debe llamarse a Google Calendar")
}

func TestGoogleCalendarSync_FullLifecycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mock := newMockCalendarClient()
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, mock, testStorage, billing.Config{}, nil, geocoding.NewClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doctor_cal_on", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	// Simulamos que el doctor ya conectó su cuenta de Google.
	require.NoError(t, db.Model(&models.Doctor{}).Where("id = ?", doc.ID).
		Update("google_calendar_refresh_token", "fake-refresh-token").Error)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "ConCalendar", "email": "concal@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	// 1. Crear la cita -> debe crear un evento.
	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z", "service": "Consulta con calendario", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))
	assert.Equal(t, 1, mock.createdCount(), "con refresh token, crear la cita debe crear el evento en Google Calendar")

	var stored models.Appointment
	require.NoError(t, db.First(&stored, apptID).Error)
	require.NotEmpty(t, stored.GoogleEventID, "el ID del evento de Google debe quedar guardado en la cita")

	// 2. Reprogramar (cambia appointmentDateTime) -> debe actualizar el evento, no crear otro.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID), token, map[string]any{
		"appointmentDateTime": "2026-08-02T15:00:00Z",
	})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mock.updatedCount(), "reprogramar debe actualizar el evento existente")
	assert.Equal(t, 1, mock.createdCount(), "reprogramar no debe crear un evento nuevo")

	// 3. Un PUT que NO cambia la fecha (ej. marcar seguimiento) no debe tocar Calendar.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID), token, map[string]any{
		"followUpDate": "2026-08-10T10:00:00Z",
	})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mock.updatedCount(), "un cambio que no sea de fecha/hora no debe volver a tocar Calendar")

	// 4. Cancelar -> debe borrar el evento.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID)+"/cancel", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mock.deletedCount(), "cancelar debe borrar el evento de Google Calendar")
}

func TestGetCurrentDoctor_GoogleCalendarConnectedField(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, newMockCalendarClient(), testStorage, billing.Config{}, nil, geocoding.NewClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doctor_cal_field", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/me", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, false, body["googleCalendarConnected"])
	assert.NotContains(t, body, "GoogleCalendarRefreshToken", "el refresh token nunca debe exponerse al frontend")

	require.NoError(t, db.Model(&models.Doctor{}).Where("id = ?", doc.ID).
		Update("google_calendar_refresh_token", "fake-token").Error)

	w = doRequest(t, router, http.MethodGet, "/api/doctor/me", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body = decodeJSON(t, w)
	assert.Equal(t, true, body["googleCalendarConnected"])
}

func TestConnectGoogleCalendar_NotConfigured(t *testing.T) {
	db := testutil.SetupTestDB(t)
	// Config vacía: simula un servidor sin GOOGLE_CLIENT_SECRET configurado.
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, newMockCalendarClient(), testStorage, billing.Config{}, nil, geocoding.NewClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doctor_cal_notconfigured", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/google-calendar/connect", token, nil)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}
