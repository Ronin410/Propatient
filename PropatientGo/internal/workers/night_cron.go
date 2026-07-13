package workers

import (
	"log"
	"propatient-api/internal/repository" // Cambia "tu_proyecto" por tu módulo de Go real
	"time"

	"gorm.io/gorm"
)

func StartNightClosureWorker(db *gorm.DB) {
	go func() {
		for {
			now := time.Now()
			// Calculamos el tiempo exacto hasta la medianoche del siguiente día
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			duration := nextMidnight.Sub(now)

			log.Printf("[WORKER] Automatización de ausencias lista. Próximo corte automático en: %v", duration)
			time.Sleep(duration)

			// Llegada la medianoche, se ejecuta la limpieza
			err := repository.ExecNightClosure(db)
			if err != nil {
				log.Printf("[ERROR AUTOMÁTICO] No se pudieron procesar los No-Shows: %v", err)
			}

			// Amortiguador de 2 segundos para evitar ejecuciones repetidas en el mismo tick de reloj
			time.Sleep(2 * time.Second)
		}
	}()
}
