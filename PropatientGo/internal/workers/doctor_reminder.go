package workers

import (
	"context"
	"fmt"
	"log"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/models"
	"propatient-api/internal/whatsapp"

	"gorm.io/gorm"
)

// doctorReminderWindow: qué tan cerca de "ahora" tiene que estar una cita
// para avisarle al doctor que está por empezar.
const doctorReminderWindow = 60 * time.Minute

// StartDoctorReminderWorker corre en segundo plano y revisa cada 10 minutos
// si hay citas confirmadas (PENDING) que empiezan dentro de la próxima
// hora y todavía no le avisaron al doctor por WhatsApp. Intervalo más corto
// que el recordatorio al paciente (30 min) porque la ventana misma es más
// angosta (60 min vs 24h) — con un ciclo de 30 min se podría pasar de largo
// una cita que entra y sale de la ventana entre una pasada y otra.
func StartDoctorReminderWorker(db *gorm.DB, waClient whatsapp.Client) {
	if waClient == nil {
		return // sin Twilio configurado, no hay nada que hacer en este worker
	}
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		SendDueDoctorReminders(db, waClient)
		for range ticker.C {
			SendDueDoctorReminders(db, waClient)
		}
	}()
}

// SendDueDoctorReminders manda el aviso de WhatsApp al doctor y marca
// DoctorReminderSentAt para no repetirlo. Exportada para los tests.
func SendDueDoctorReminders(db *gorm.DB, waClient whatsapp.Client) {
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
		if err := db.First(&doctor, appt.DoctorID).Error; err != nil || doctor.Phone == "" {
			continue
		}

		patientName := "un paciente"
		if appt.Patient != nil {
			patientName = fmt.Sprintf("%s %s", appt.Patient.FirstName, appt.Patient.LastName)
		}
		body := fmt.Sprintf(
			"Recordatorio: tu cita con %s es a las %s. — ProPatient",
			patientName, auth.FormatSpanishDateTime(appt.AppointmentDateTime),
		)

		if err := waClient.SendMessage(context.Background(), doctor.Phone, body); err != nil {
			log.Printf("⚠️ No se pudo enviar el recordatorio al doctor de la cita %d: %v", appt.ID, err)
			continue
		}

		sentAt := time.Now().UTC()
		if err := db.Model(&models.Appointment{}).Where("id = ?", appt.ID).Update("doctor_reminder_sent_at", sentAt).Error; err != nil {
			log.Printf("⚠️ No se pudo marcar el recordatorio al doctor como enviado (cita %d): %v", appt.ID, err)
		}
	}
}
