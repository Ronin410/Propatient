package workers

import (
	"log"
	"propatient-api/internal/repository"
	"time"

	"gorm.io/gorm"
)

// nightClosureCheckInterval: cada cuánto se revisa si hay citas PENDING
// vencidas. Antes este worker calculaba el tiempo exacto hasta la
// medianoche y hacía un único time.Sleep() de varias horas — en Render
// (plan free), el contenedor se "duerme" tras un rato sin tráfico HTTP, lo
// que MATA el proceso completo, no solo pausa la goroutine. Ese sleep de
// horas casi nunca llegaba a completarse, así que el corte de No-Shows
// dejaba de ejecutarse en la práctica. Un ticker corto (mismo patrón que
// StartAppointmentReminderWorker) es robusto a esos reinicios: en cuanto
// el contenedor despierta por cualquier petición, el worker vuelve a
// arrancar y el siguiente corte corre en minutos, no en horas.
const nightClosureCheckInterval = 15 * time.Minute

func StartNightClosureWorker(db *gorm.DB) {
	go func() {
		ticker := time.NewTicker(nightClosureCheckInterval)
		defer ticker.Stop()

		runNightClosure(db)
		for range ticker.C {
			runNightClosure(db)
		}
	}()
}

func runNightClosure(db *gorm.DB) {
	if err := repository.ExecNightClosure(db); err != nil {
		log.Printf("[ERROR AUTOMÁTICO] No se pudieron procesar los No-Shows: %v", err)
	}
}
