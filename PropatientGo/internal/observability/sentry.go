// Package observability inicializa el reporte de errores a Sentry. Sin
// SENTRY_DSN configurada, InitSentry no hace nada — el resto de la app
// funciona exactamente igual, solo sin reportar errores, mismo patrón que
// el resto de integraciones opcionales de este proyecto (S3, WhatsApp,
// Google Calendar, reCAPTCHA).
package observability

import (
	"log"
	"os"

	sentrygo "github.com/getsentry/sentry-go"
)

// InitSentry se llama una sola vez, al arrancar el proceso (ver main.go).
// SENTRY_ENVIRONMENT permite distinguir errores de producción vs. de un
// entorno de pruebas apuntando al mismo proyecto de Sentry; sin definirla,
// asume "production" (el caso normal en Render).
func InitSentry() {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		log.Println("⚠️ Sentry NO configurado — falta SENTRY_DSN. Los errores del backend no se reportarán a Sentry.")
		return
	}

	environment := os.Getenv("SENTRY_ENVIRONMENT")
	if environment == "" {
		environment = "production"
	}

	err := sentrygo.Init(sentrygo.ClientOptions{
		Dsn:         dsn,
		Environment: environment,
		// 20% de las peticiones, no 100%: suficiente para detectar
		// endpoints lentos sin generar un volumen de trazas que rebase la
		// cuota gratuita solo por tráfico normal.
		TracesSampleRate: 0.2,
	})
	if err != nil {
		log.Printf("⚠️ No se pudo inicializar Sentry: %v", err)
		return
	}
	log.Println("✅ Sentry configurado — los errores y panics del backend se reportarán automáticamente.")
}
