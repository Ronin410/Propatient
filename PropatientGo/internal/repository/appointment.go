package repository

import (
	"log"
	"propatient-api/internal/auth"
	"time"

	"gorm.io/gorm"
)

// ExecNightClosure busca citas PENDING de días YA TERMINADOS (no las de
// hoy, sin importar la hora que ya pasó) y las cambia a NOSHOW.
//
// A propósito NO usa "ya pasó la hora de la cita" como corte: si un
// paciente llega tarde, su cita debe seguir visible y accionable en el
// dashboard (Iniciar Atención / Reprogramar) durante todo ese día — solo
// se convierte en NOSHOW automático una vez que el día calendario (zona
// horaria del consultorio, ver auth.AppTimeZone) ya terminó por completo.
// Este worker sigue corriendo cada 15 min (ver StartNightClosureWorker),
// pero el filtro de abajo hace que en la práctica solo tenga efecto una
// vez que cruza la medianoche — nunca durante el mismo día de la cita.
func ExecNightClosure(db *gorm.DB) error {
	loc, err := time.LoadLocation(auth.AppTimeZone)
	if err != nil {
		loc = time.UTC
	}
	y, m, d := time.Now().In(loc).Date()
	startOfToday := time.Date(y, m, d, 0, 0, 0, 0, loc)

	// Nota: la columna real es "appointment_date_time" (ver models.Appointment);
	// el nombre anterior no existía y hacía fallar este UPDATE todas las noches.
	result := db.Table("appointments").
		Where("status IN ? AND appointment_date_time < ?", []string{"PENDING", "pending"}, startOfToday).
		Update("status", "NOSHOW")

	if result.Error != nil {
		return result.Error
	}

	log.Printf("[CRON AUTOMÁTICO] GORM ejecutó el corte. %d citas inactivas cambiadas a 'NOSHOW'", result.RowsAffected)
	return nil
}
