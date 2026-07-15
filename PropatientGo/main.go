package main

import (
	"log"
	"os"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/database"
	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/whatsapp"
	"propatient-api/internal/workers"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1. Cargar variables de entorno
	//Otro comentairio
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró archivo .env, usando variables de entorno del sistema")
	}

	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("Error crítico: la variable de entorno JWT_SECRET no está definida")
	}

	// 2. Conexión a DB
	dsn := os.Getenv("DATABASE_URL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		log.Fatal("Error crítico al conectar a la DB:", err)
	}

	// 3. Automigración y Seed
	db.AutoMigrate(&models.Doctor{}, &models.Patient{}, &models.MedicalHistory{}, &models.Appointment{}, &models.MedicalDocument{}, &models.DoctorTemplate{}, &models.Staff{})

	// Limpieza de compatibilidad: patients.email ya no debe ser único a nivel
	// de base de datos (un mismo paciente puede estar vinculado a varios
	// doctores, y dos pacientes sin correo colisionaban en "" y no se podían
	// crear). AutoMigrate no elimina índices existentes solo por quitar el
	// tag "unique" del struct, así que se elimina explícitamente si quedó de
	// un despliegue anterior. Silencioso si ya no existe.
	if db.Migrator().HasIndex(&models.Patient{}, "Email") {
		if err := db.Migrator().DropIndex(&models.Patient{}, "Email"); err != nil {
			log.Println("Aviso: no se pudo eliminar el índice único viejo de patients.email:", err)
		}
	}

	// Migración de compatibilidad para el cobro de suscripción: los doctores
	// que ya existían antes de esta funcionalidad no tienen TrialEndsAt (el
	// campo es nuevo). Sin este backfill, billing.RequireActiveSubscription
	// los bloquearía con 402 en el primer request después de este deploy —
	// a un doctor que ya estaba usando la app normalmente. Se les da un
	// periodo de gracia de 30 días desde este deploy para que puedan
	// suscribirse con tiempo de sobra.
	if err := db.Exec(
		"UPDATE doctors SET subscription_status = 'trialing', trial_ends_at = NOW() + INTERVAL '30 days' WHERE trial_ends_at IS NULL",
	).Error; err != nil {
		log.Println("Aviso: no se pudo aplicar el periodo de gracia de suscripción a doctores existentes:", err)
	}

	database.SeedDatabase(db)

	// Cliente de WhatsApp (Twilio): solo se construye si las tres
	// variables están configuradas. Sin eso, los workers de recordatorio
	// simplemente no mandan WhatsApp (el correo sigue funcionando igual).
	whatsappConfig := whatsapp.LoadConfigFromEnv()
	var whatsappClient whatsapp.Client
	if whatsappConfig.IsConfigured() {
		whatsappClient = whatsapp.NewClient(whatsappConfig)
	}

	workers.StartNightClosureWorker(db)
	workers.StartAppointmentReminderWorker(db, auth.SendEmail, whatsappClient)
	workers.StartDoctorReminderWorker(db, whatsappClient)

	// 4. Configuración del Router (rutas, CORS, health check en internal/server)
	r := server.NewRouter(db)

	// 5. Lanzar Servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "8095"
	}

	log.Printf("🚀 ProPatient API (Go) corriendo en puerto %s", port)
	r.Run(":" + port)
}
