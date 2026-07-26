package workers_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"propatient-api/internal/models"
	"propatient-api/internal/testutil"
	"propatient-api/internal/whatsapp"
	"propatient-api/internal/workers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// recordingSender simula auth.SendEmail sin tocar la red: guarda cada
// llamada para poder verificarla, y puede configurarse para fallar.
type recordingSender struct {
	mu    sync.Mutex
	calls []string
	fail  bool
}

func (r *recordingSender) send(toEmail, subject, htmlBody string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("fallo simulado de envío")
	}
	r.calls = append(r.calls, toEmail)
	return nil
}

func (r *recordingSender) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// recordingWhatsApp simula whatsapp.Client sin tocar la red.
type recordingWhatsApp struct {
	mu    sync.Mutex
	calls []string
	fail  bool
}

func (r *recordingWhatsApp) SendMessage(ctx context.Context, toPhone, body string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("fallo simulado de envío")
	}
	r.calls = append(r.calls, toPhone)
	return nil
}

func (r *recordingWhatsApp) SendTemplate(ctx context.Context, toPhone, contentSID string, variables map[string]string) error {
	return r.SendMessage(ctx, toPhone, contentSID)
}

func (r *recordingWhatsApp) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func createReminderTestAppointment(t *testing.T, db *gorm.DB, doctorID uint, patientEmail string, when time.Time, status string) uint {
	t.Helper()
	patient := models.Patient{FirstName: "Paciente", LastName: "Prueba", Email: patientEmail, Phone: "5550001111"}
	require.NoError(t, db.Create(&patient).Error)

	appt := models.Appointment{
		PatientID:           patient.ID,
		DoctorID:            doctorID,
		AppointmentDateTime: when,
		Status:              status,
	}
	require.NoError(t, db.Create(&appt).Error)
	return appt.ID
}

// TestSendDueAppointmentReminders_SendsForUpcomingWithinWindow confirma el
// caso central: una cita PENDING dentro de las próximas 24h con paciente
// con correo recibe el recordatorio y queda marcada para no repetirse.
func TestSendDueAppointmentReminders_SendsForUpcomingWithinWindow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_reminder_due", "password123")

	apptID := createReminderTestAppointment(t, db, doc.ID, "paciente.recordatorio@test.local",
		time.Now().UTC().Add(10*time.Hour), "PENDING")

	sender := &recordingSender{}
	workers.SendDueAppointmentReminders(db, sender.send, nil, whatsapp.Templates{})

	assert.Equal(t, 1, sender.count())
	assert.Equal(t, "paciente.recordatorio@test.local", sender.calls[0])

	var appt models.Appointment
	require.NoError(t, db.First(&appt, apptID).Error)
	require.NotNil(t, appt.ReminderSentAt)

	// Una segunda pasada del worker no debe reenviarlo.
	workers.SendDueAppointmentReminders(db, sender.send, nil, whatsapp.Templates{})
	assert.Equal(t, 1, sender.count(), "no debe reenviar el recordatorio una vez marcado")
}

// TestSendDueAppointmentReminders_SkipsOutsideWindowOrWrongStatus confirma
// que no manda recordatorios para citas fuera de la ventana de 24h ni para
// solicitudes públicas aún sin confirmar.
func TestSendDueAppointmentReminders_SkipsOutsideWindowOrWrongStatus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_reminder_skip", "password123")

	// Muy lejos en el futuro (fuera de la ventana de 24h).
	createReminderTestAppointment(t, db, doc.ID, "lejos@test.local", time.Now().UTC().Add(72*time.Hour), "PENDING")
	// Solicitud pública sin confirmar todavía.
	createReminderTestAppointment(t, db, doc.ID, "sin-confirmar@test.local", time.Now().UTC().Add(5*time.Hour), "PENDING_CONFIRMATION")
	// Ya pasada / cancelada.
	createReminderTestAppointment(t, db, doc.ID, "cancelada@test.local", time.Now().UTC().Add(5*time.Hour), "CANCELLED")

	sender := &recordingSender{}
	wa := &recordingWhatsApp{}
	workers.SendDueAppointmentReminders(db, sender.send, wa, whatsapp.Templates{})

	assert.Equal(t, 0, sender.count(), "ninguna de estas citas debía disparar un recordatorio por correo")
	assert.Equal(t, 0, wa.count(), "ninguna de estas citas debía disparar un recordatorio por WhatsApp")
}

// TestSendDueAppointmentReminders_RetriesAfterSendFailure confirma que si
// el envío falla (ej. SMTP no configurado y sin WhatsApp), la cita se queda
// sin marcar para reintentarse en la siguiente pasada, en vez de darse por
// enviada.
func TestSendDueAppointmentReminders_RetriesAfterSendFailure(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_reminder_retry", "password123")

	apptID := createReminderTestAppointment(t, db, doc.ID, "reintento@test.local",
		time.Now().UTC().Add(10*time.Hour), "PENDING")

	failingSender := &recordingSender{fail: true}
	workers.SendDueAppointmentReminders(db, failingSender.send, nil, whatsapp.Templates{})

	var appt models.Appointment
	require.NoError(t, db.First(&appt, apptID).Error)
	assert.Nil(t, appt.ReminderSentAt, "no debe marcarse como enviado si el envío falló")

	workingSender := &recordingSender{}
	workers.SendDueAppointmentReminders(db, workingSender.send, nil, whatsapp.Templates{})
	assert.Equal(t, 1, workingSender.count(), "debe reintentar en la siguiente pasada")
}

// TestSendDueAppointmentReminders_SkipsEmailWhenWhatsAppSucceeds confirma
// que, con un cliente de WhatsApp configurado y funcionando, el recordatorio
// sale SOLO por WhatsApp — el correo se salta para no duplicar el aviso
// (WhatsApp es el canal principal, el correo queda como respaldo).
func TestSendDueAppointmentReminders_SkipsEmailWhenWhatsAppSucceeds(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_reminder_wa", "password123")

	createReminderTestAppointment(t, db, doc.ID, "wa.recordatorio@test.local",
		time.Now().UTC().Add(10*time.Hour), "PENDING")

	sender := &recordingSender{}
	wa := &recordingWhatsApp{}
	workers.SendDueAppointmentReminders(db, sender.send, wa, whatsapp.Templates{})

	assert.Equal(t, 0, sender.count(), "el correo no debe mandarse si el WhatsApp ya se entregó")
	assert.Equal(t, 1, wa.count())
	assert.Equal(t, "5550001111", wa.calls[0])
}

// TestSendDueAppointmentReminders_FallsBackToEmailWhenWhatsAppFails cubre
// el caso en que WhatsApp se intenta pero falla: el correo debe usarse como
// respaldo para que el paciente sí reciba el recordatorio por algún canal.
func TestSendDueAppointmentReminders_FallsBackToEmailWhenWhatsAppFails(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_reminder_mixed", "password123")

	apptID := createReminderTestAppointment(t, db, doc.ID, "correo-respaldo@test.local",
		time.Now().UTC().Add(10*time.Hour), "PENDING")

	sender := &recordingSender{}
	wa := &recordingWhatsApp{fail: true}
	workers.SendDueAppointmentReminders(db, sender.send, wa, whatsapp.Templates{})

	assert.Equal(t, 1, sender.count(), "el correo debe usarse como respaldo si el WhatsApp falla")

	var appt models.Appointment
	require.NoError(t, db.First(&appt, apptID).Error)
	assert.NotNil(t, appt.ReminderSentAt, "con al menos un canal exitoso, no debe reintentarse")
}
