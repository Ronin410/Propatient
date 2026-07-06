package repository

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// ExecNightClosure busca citas PENDING que ya pasaron (o son de hoy) y las cambia a NOSHOW
func ExecNightClosure(db *gorm.DB) error {
	// Obtenemos la fecha de hoy a las 23:59:59 para mitigar problemas de huso horario
	// O simplemente comparamos con el inicio del día siguiente.
	now := time.Now()

	// Filtramos por estatus PENDING y que la fecha sea menor o igual al día de hoy
	result := db.Table("appointments").
		Where("status IN ? AND appointment_date <= ?", []string{"PENDING", "pending"}, now).
		Update("status", "NOSHOW")

	if result.Error != nil {
		return result.Error
	}

	log.Printf("[CRON AUTOMÁTICO] GORM ejecutó el corte. %d citas inactivas cambiadas a 'NOSHOW'", result.RowsAffected)
	return nil
}
