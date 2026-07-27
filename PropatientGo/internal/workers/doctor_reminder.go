package workers

import (
	"fmt"
	"log"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/models"

	"gorm.io/gorm"
)

// doctorReminderWindow: qué tan cerca de "ahora" tiene que estar una cita
// para avisarle al doctor que está por empezar.
const doctorReminderWindow = 60 * time.Minute

// StartDoctorReminderWorker corre en segundo plano y revisa cada 10 minutos
// si hay citas confirmadas (PENDING) que empiezan dentro de la próxima
// hora y todavía no le avisaron al doctor. Va por correo (no WhatsApp): el
// doctor ya tiene que entrar a la app para iniciar la consulta, así que
// este aviso es el que menos justifica el costo de un mensaje de WhatsApp
// de los seis que manda la plataforma — moverlo a correo (gratis, ver
// internal/auth.SendEmail/Resend) no le quita valor real. Intervalo más
// corto que el recordatorio al paciente (30 min) porque la ventana misma es
// más angosta (60 min vs 24h) — con un ciclo de 30 min se podría pasar de
// largo una cita que entra y sale de la ventana entre una pasada y otra.
func StartDoctorReminderWorker(db *gorm.DB, sendEmail EmailSender) {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		SendDueDoctorReminders(db, sendEmail)
		for range ticker.C {
			SendDueDoctorReminders(db, sendEmail)
		}
	}()
}

// SendDueDoctorReminders manda el aviso por correo al doctor y marca
// DoctorReminderSentAt para no repetirlo. Exportada para los tests.
func SendDueDoctorReminders(db *gorm.DB, sendEmail EmailSender) {
	now := time.Now().UTC()
	until := now.Add(doctorReminderWindow)

	var appointments []models.Appointment
	if err := db.Preload("Patient").
		Where("status = ? AND doctor_reminder_sent_at IS NULL AND appointment_date_time BETWEEN ? AND ?", "PENDING", now, until).
		Find(&appointments).Error; err != nil {
		log.Printf("⚠️ No se pudieron consultar las citas pendientes de recordatorio al doctor: %v", err)
		return
	}

	for _, appt := range appointments {
		var doctor models.Doctor
		if err := db.First(&doctor, appt.DoctorID).Error; err != nil || doctor.Email == "" {
			continue
		}

		patientName := "un paciente"
		if appt.Patient != nil {
			patientName = fmt.Sprintf("%s %s", appt.Patient.FirstName, appt.Patient.LastName)
		}
		when := auth.FormatSpanishDateTime(appt.AppointmentDateTime)
		subject := "Recordatorio: tu cita con " + patientName + " está por comenzar"
		body := fmt.Sprintf(
			`<p>Recordatorio: tu cita con <strong>%s</strong> es a las <strong>%s</strong>.</p>
			<p>— ProPatient Clinic</p>`,
			patientName, when,
		)

		if err := sendEmail(doctor.Email, subject, body); err != nil {
			log.Printf("⚠️ No se pudo enviar el recordatorio al doctor de la cita %d: %v", appt.ID, err)
			continue
		}

		sentAt := time.Now().UTC()
		if err := db.Model(&models.Appointment{}).Where("id = ?", appt.ID).Update("doctor_reminder_sent_at", sentAt).Error; err != nil {
			log.Printf("⚠️ No se pudo marcar el recordatorio al doctor como enviado (cita %d): %v", appt.ID, err)
		}
	}
}
