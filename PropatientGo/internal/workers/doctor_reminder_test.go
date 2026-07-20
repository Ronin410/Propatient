package workers_test

import (
	"testing"
	"time"

	"propatient-api/internal/models"
	"propatient-api/internal/testutil"
	"propatient-api/internal/workers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createDoctorReminderTestAppointment(t *testing.T, db *gorm.DB, doctorID uint, when time.Time, status string) uint {
	t.Helper()
	patient := models.Patient{FirstName: "Carlos", LastName: "Ramírez", Phone: "5559998888"}
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

// TestSendDueDoctorReminders_SendsWithinWindow confirma el caso central: el
// doctor recibe un correo para una cita PENDING dentro de la próxima hora,
// y queda marcada para no repetirse. Va por correo (no WhatsApp) porque el
// doctor ya tiene que entrar a la app para iniciar la consulta.
func TestSendDueDoctorReminders_SendsWithinWindow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_reminder_dr", "password123")

	apptID := createDoctorReminderTestAppointment(t, db, doc.ID, time.Now().UTC().Add(30*time.Minute), "PENDING")

	sender := &recordingSender{}
	workers.SendDueDoctorReminders(db, sender.send)

	require.Equal(t, 1, sender.count())
	assert.Equal(t, doc.Email, sender.calls[0])

	var appt models.Appointment
	require.NoError(t, db.First(&appt, apptID).Error)
	require.NotNil(t, appt.DoctorReminderSentAt)

	workers.SendDueDoctorReminders(db, sender.send)
	assert.Equal(t, 1, sender.count(), "no debe reenviarlo una vez marcado")
}

// TestSendDueDoctorReminders_SkipsOutsideWindowWrongStatusOrNoEmail cubre
// los casos que no deben disparar nada: fuera de la ventana de 60 min,
// solicitud pública sin confirmar, y doctor sin correo registrado.
func TestSendDueDoctorReminders_SkipsOutsideWindowWrongStatusOrNoEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	docWithEmail := testutil.CreateTestDoctor(t, db, "doc_dr_skip_far", "password123")
	docNoEmail := testutil.CreateTestDoctor(t, db, "doc_dr_skip_noemail", "password123")
	require.NoError(t, db.Model(&docNoEmail).Update("email", "").Error)

	// Fuera de la ventana de 60 minutos.
	createDoctorReminderTestAppointment(t, db, docWithEmail.ID, time.Now().UTC().Add(3*time.Hour), "PENDING")
	// Solicitud pública sin confirmar todavía.
	createDoctorReminderTestAppointment(t, db, docWithEmail.ID, time.Now().UTC().Add(20*time.Minute), "PENDING_CONFIRMATION")
	// Doctor sin correo registrado.
	createDoctorReminderTestAppointment(t, db, docNoEmail.ID, time.Now().UTC().Add(20*time.Minute), "PENDING")

	sender := &recordingSender{}
	workers.SendDueDoctorReminders(db, sender.send)

	assert.Equal(t, 0, sender.count())
}

// TestSendDueDoctorReminders_RetriesAfterSendFailure confirma que un
// fallo de envío no marca la cita como avisada.
func TestSendDueDoctorReminders_RetriesAfterSendFailure(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_reminder_dr_retry", "password123")

	apptID := createDoctorReminderTestAppointment(t, db, doc.ID, time.Now().UTC().Add(15*time.Minute), "PENDING")

	failingSender := &recordingSender{fail: true}
	workers.SendDueDoctorReminders(db, failingSender.send)

	var appt models.Appointment
	require.NoError(t, db.First(&appt, apptID).Error)
	assert.Nil(t, appt.DoctorReminderSentAt)

	workingSender := &recordingSender{}
	workers.SendDueDoctorReminders(db, workingSender.send)
	assert.Equal(t, 1, workingSender.count(), "debe reintentar en la siguiente pasada")
}
