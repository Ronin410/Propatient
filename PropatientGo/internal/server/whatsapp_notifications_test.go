package server_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"propatient-api/internal/billing"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/storage"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublicAppointment_NotifiesPatientAndDoctorByWhatsApp confirma que,
// con Twilio configurado, tanto el paciente como el doctor reciben un
// WhatsApp al mandarse una solicitud de cita pública.
func TestPublicAppointment_NotifiesPatientAndDoctorByWhatsApp(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	wa := newMockWhatsAppClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), wa, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_wa_booking", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"public_listed": true, "public_slug": "dr-wa-booking-1", "phone": "5550009999",
	}).Error)

	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Sofía",
		"patientLastName":     "Nuñez",
		"patientPhone":        "5551237890",
		"patientEmail":        "sofia.wa@test.local",
		"dataConsent":         true,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// El envío va en segundo plano (no bloquea la respuesta HTTP), así que
	// hay que darle tiempo antes de verificar.
	wa.waitForCallsTo(t, "5551237890", 1)
	wa.waitForCallsTo(t, "5550009999", 1)
}

// TestConfirmAppointment_NotifiesPatientByWhatsApp confirma que al aceptar
// una solicitud, el paciente recibe un WhatsApp de confirmación.
func TestConfirmAppointment_NotifiesPatientByWhatsApp(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	wa := newMockWhatsAppClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), wa, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_wa_confirm", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-wa-confirm-1"}).Error)
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Iván",
		"patientLastName":     "Castro",
		"patientPhone":        "5559990000",
		"patientEmail":        "ivan.wa@test.local",
		"dataConsent":         true,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=PENDING_CONFIRMATION", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var requests []map[string]any
	decodeJSONList(t, w, &requests)
	require.Len(t, requests, 1)
	apptID := strconv.Itoa(int(requests[0]["id"].(float64)))

	// La solicitud pública ya le mandó 1 WhatsApp de confirmación de
	// solicitud a este mismo teléfono; confirmar debe sumarle uno más.
	wa.waitForCallsTo(t, "5559990000", 1)

	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+apptID+"/confirm", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)

	wa.waitForCallsTo(t, "5559990000", 2)

	// El WhatsApp de confirmación debe incluir el link de "sube tus
	// documentos antes de la consulta" — antes de este cambio, una cita
	// que llegaba por el directorio público (a diferencia de una agendada
	// directo por el doctor) nunca lo incluía.
	var uploadToken string
	require.NoError(t, db.Raw("SELECT upload_token FROM appointments WHERE id = ?", apptID).Scan(&uploadToken).Error)
	require.NotEmpty(t, uploadToken, "ConfirmAppointment debe generar el upload_token para poder mandarlo en el WhatsApp")

	lastBody := wa.lastBodyTo(t, "5559990000")
	assert.Contains(t, lastBody, "/public-upload/"+uploadToken)
}

// TestCancelAppointment_NotifiesPatientWithDistinctWordingByCase cubre las
// dos ramas: rechazar una solicitud PENDING_CONFIRMATION manda el aviso de
// "no pudimos aceptar tu solicitud"; cancelar una cita ya confirmada
// (PENDING normal) manda un aviso distinto de "tu cita fue cancelada" — en
// ambos casos el paciente debe quedar notificado por WhatsApp.
func TestCancelAppointment_NotifiesPatientWithDistinctWordingByCase(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	wa := newMockWhatsAppClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), wa, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_wa_reject", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-wa-reject-1"}).Error)
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	// Solicitud pública que se va a rechazar.
	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Rechazado",
		"patientLastName":     "Prueba",
		"patientPhone":        "5551110001",
		"patientEmail":        "rechazado.wa@test.local",
		"dataConsent":         true,
	})
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=PENDING_CONFIRMATION", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var requests []map[string]any
	decodeJSONList(t, w, &requests)
	require.Len(t, requests, 1)
	rejectID := strconv.Itoa(int(requests[0]["id"].(float64)))

	wa.waitForCallsTo(t, "5551110001", 1) // ya tenía 1 de la solicitud pública
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+rejectID+"/cancel", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	wa.waitForCallsTo(t, "5551110001", 2)

	// Cita normal ya confirmada (creada directo, no como solicitud pública) que se cancela.
	patient := models.Patient{FirstName: "Normal", LastName: "Prueba", Phone: "5552220002", Email: "normal.wa@test.local"}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Model(&doc).Association("Patients").Append(&patient))
	appt := models.Appointment{PatientID: patient.ID, DoctorID: doc.ID, AppointmentDateTime: time.Now().UTC().Add(48 * time.Hour), Status: "PENDING"}
	require.NoError(t, db.Create(&appt).Error)

	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(int(appt.ID))+"/cancel", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	wa.waitForCallsTo(t, "5552220002", 1)
}

// TestUpdateAppointment_NotifiesPatientWhenFollowUpDateSet cubre el aviso
// de seguimiento: se manda solo cuando FollowUpDate cambia a un valor
// nuevo, no en cada guardado.
func TestUpdateAppointment_NotifiesPatientWhenFollowUpDateSet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	wa := newMockWhatsAppClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), wa, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_wa_followup", "password123")
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	patient := models.Patient{FirstName: "Seguimiento", LastName: "Prueba", Phone: "5553330003", Email: "seguimiento.wa@test.local"}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Model(&doc).Association("Patients").Append(&patient))
	appt := models.Appointment{PatientID: patient.ID, DoctorID: doc.ID, AppointmentDateTime: time.Now().UTC().Add(-time.Hour), Status: "PENDING"}
	require.NoError(t, db.Create(&appt).Error)

	followUp := time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	w := doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(int(appt.ID)), docToken, map[string]any{
		"followUpDate": followUp,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	wa.waitForCallsTo(t, "5553330003", 1)

	// Guardar de nuevo con la MISMA fecha de seguimiento no debe repetir el aviso.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(int(appt.ID)), docToken, map[string]any{
		"followUpDate": followUp,
	})
	require.Equal(t, http.StatusOK, w.Code)
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 1, wa.callsTo("5553330003"), "no debe repetirse si la fecha de seguimiento no cambió")
}

// TestUpdateAppointment_NotifiesPatientWhenRescheduled cubre el aviso nuevo
// de reprogramación: cambiar appointmentDateTime en una cita PENDING debe
// avisarle al paciente por WhatsApp; guardar sin tocar la fecha (ej. solo
// notas) no debe disparar nada.
func TestUpdateAppointment_NotifiesPatientWhenRescheduled(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	wa := newMockWhatsAppClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), wa, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_wa_reschedule", "password123")
	require.NoError(t, db.Model(&doc).Update("phone", "5559876543").Error)
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	patient := models.Patient{FirstName: "Reprogramado", LastName: "Prueba", Phone: "5554440004", Email: "reprogramado.wa@test.local"}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Model(&doc).Association("Patients").Append(&patient))
	appt := models.Appointment{PatientID: patient.ID, DoctorID: doc.ID, AppointmentDateTime: time.Now().UTC().Add(48 * time.Hour), Status: "PENDING"}
	require.NoError(t, db.Create(&appt).Error)

	// Guardar sin cambiar la fecha (solo notas) no debe avisar nada.
	w := doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(int(appt.ID)), docToken, map[string]any{
		"notes": "Nota sin reprogramar",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 0, wa.callsTo("5554440004"), "no debe avisar si la fecha/hora no cambió")

	newDateTime := time.Now().UTC().Add(72 * time.Hour).Format(time.RFC3339)
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(int(appt.ID)), docToken, map[string]any{
		"appointmentDateTime": newDateTime,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	wa.waitForCallsTo(t, "5554440004", 1)

	// Debe invitar al paciente a comunicarse con el doctor (con el teléfono
	// que este cargó en su perfil) si no está de acuerdo con el cambio.
	body := wa.lastBodyTo(t, "5554440004")
	assert.Contains(t, body, "no estás de acuerdo con la reprogramación")
	assert.Contains(t, body, "5559876543")
}
