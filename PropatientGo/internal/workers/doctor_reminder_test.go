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
// doctor recibe un WhatsApp para una cita PENDING dentro de la próxima
// hora, y queda marcada para no repetirse.
func TestSendDueDoctorReminders_SendsWithinWindow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_reminder_dr", "password123")
	require.NoError(t, db.Model(&doc).Update("phone", "5551230000").Error)

	apptID := createDoctorReminderTestAppointment(t, db, doc.ID, time.Now().UTC().Add(30*time.Minute), "PENDING")

	wa := &recordingWhatsApp{}
	workers.SendDueDoctorReminders(db, wa)

	require.Equal(t, 1, wa.count())
	assert.Equal(t, "5551230000", wa.calls[0])

	var appt models.Appointment
	require.NoError(t, db.First(&appt, apptID).Error)
	require.NotNil(t, appt.DoctorReminderSentAt)

	workers.SendDueDoctorReminders(db, wa)
	assert.Equal(t, 1, wa.count(), "no debe reenviarlo una vez marcado")
}

// TestSendDueDoctorReminders_SkipsOutsideWindowWrongStatusOrNoPhone cubre
// los casos que no deben disparar nada: fuera de la ventana de 60 min,
// solicitud pública sin confirmar, y doctor sin teléfono registrado.
func TestSendDueDoctorReminders_SkipsOutsideWindowWrongStatusOrNoPhone(t *testing.T) {
	db := testutil.SetupTestDB(t)
	docWithPhone := testutil.CreateTestDoctor(t, db, "doc_dr_skip_far", "password123")
	require.NoError(t, db.Model(&docWithPhone).Update("phone", "5551110000").Error)
	docNoPhone := testutil.CreateTestDoctor(t, db, "doc_dr_skip_nophone", "password123")

	// Fuera de la ventana de 60 minutos.
	createDoctorReminderTestAppointment(t, db, docWithPhone.ID, time.Now().UTC().Add(3*time.Hour), "PENDING")
	// Solicitud pública sin confirmar todavía.
	createDoctorReminderTestAppointment(t, db, docWithPhone.ID, time.Now().UTC().Add(20*time.Minute), "PENDING_CONFIRMATION")
	// Doctor sin teléfono registrado.
	createDoctorReminderTestAppointment(t, db, docNoPhone.ID, time.Now().UTC().Add(20*time.Minute), "PENDING")

	wa := &recordingWhatsApp{}
	workers.SendDueDoctorReminders(db, wa)

	assert.Equal(t, 0, wa.count())
}

// TestSendDueDoctorReminders_RetriesAfterSendFailure confirma que un
// fallo de envío no marca la cita como avisada.
func TestSendDueDoctorReminders_RetriesAfterSendFailure(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_reminder_dr_retry", "password123")
	require.NoError(t, db.Model(&doc).Update("phone", "5559990000").Error)

	apptID := createDoctorReminderTestAppointment(t, db, doc.ID, time.Now().UTC().Add(15*time.Minute), "PENDING")

	failingWA := &recordingWhatsApp{fail: true}
	workers.SendDueDoctorReminders(db, failingWA)

	var appt models.Appointment
	require.NoError(t, db.First(&appt, apptID).Error)
	assert.Nil(t, appt.DoctorReminderSentAt)

	workingWA := &recordingWhatsApp{}
	workers.SendDueDoctorReminders(db, workingWA)
	assert.Equal(t, 1, workingWA.count(), "debe reintentar en la siguiente pasada")
}
