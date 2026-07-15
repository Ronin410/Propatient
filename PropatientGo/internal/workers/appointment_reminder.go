package workers

import (
	"fmt"
	"log"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/models"

	"gorm.io/gorm"
)

// reminderWindow: qué tan cerca de "ahora" tiene que estar una cita para
// que se considere lista para su recordatorio.
const reminderWindow = 24 * time.Hour

// EmailSender tiene la misma forma que auth.SendEmail — inyectable para
// poder probar el worker sin depender de credenciales SMTP reales, mismo
// patrón que los demás clientes externos de la app (geocoding, storage,
// billing, googlecalendar).
type EmailSender func(toEmail, subject, htmlBody string) error

// StartAppointmentReminderWorker corre en segundo plano y revisa cada 30
// minutos si hay citas confirmadas (PENDING) que empiezan dentro de las
// próximas 24 horas y todavía no tienen su recordatorio enviado.
func StartAppointmentReminderWorker(db *gorm.DB, sendEmail EmailSender) {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		SendDueAppointmentReminders(db, sendEmail)
		for range ticker.C {
			SendDueAppointmentReminders(db, sendEmail)
		}
	}()
}

// SendDueAppointmentReminders manda el correo de recordatorio a cada cita
// que lo necesite y marca ReminderSentAt para no repetirlo en la siguiente
// pasada. Exportada para poder llamarla directo desde los tests sin
// esperar al ticker.
func SendDueAppointmentReminders(db *gorm.DB, sendEmail EmailSender) {
	now := time.Now().UTC()
	until := now.Add(reminderWindow)

	var appointments []models.Appointment
	if err := db.Preload("Patient").
		Where("status = ? AND reminder_sent_at IS NULL AND appointment_date_time BETWEEN ? AND ?", "PENDING", now, until).
		Find(&appointments).Error; err != nil {
		log.Printf("⚠️ No se pudieron consultar las citas pendientes de recordatorio: %v", err)
		return
	}

	for _, appt := range appointments {
		if appt.Patient == nil || appt.Patient.Email == "" {
			continue
		}

		var doctor models.Doctor
		if err := db.First(&doctor, appt.DoctorID).Error; err != nil {
			continue
		}

		subject := "Recordatorio: tu cita con Dr(a). " + doctor.FullName
		body := fmt.Sprintf(
			`<p>Hola %s,</p>
			<p>Te recordamos tu cita con <strong>Dr(a). %s</strong> el <strong>%s</strong>.</p>
			<p>— ProPatient</p>`,
			appt.Patient.FirstName, doctor.FullName, auth.FormatSpanishDateTime(appt.AppointmentDateTime),
		)

		if err := sendEmail(appt.Patient.Email, subject, body); err != nil {
			log.Printf("⚠️ No se pudo enviar el recordatorio de la cita %d: %v", appt.ID, err)
			continue
		}

		sentAt := time.Now().UTC()
		if err := db.Model(&models.Appointment{}).Where("id = ?", appt.ID).Update("reminder_sent_at", sentAt).Error; err != nil {
			log.Printf("⚠️ No se pudo marcar el recordatorio como enviado (cita %d): %v", appt.ID, err)
		}
	}
}
