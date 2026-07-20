package main

import (
	"log"
	"os"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/database"
	"propatient-api/internal/models"
	"propatient-api/internal/observability"
	"propatient-api/internal/server"
	"propatient-api/internal/whatsapp"
	"propatient-api/internal/workers"

	sentrygo "github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	// Empaqueta la base de datos de husos horarios (IANA) directo en el
	// binario, para que time.LoadLocation("America/Mazatlan") (ver
	// auth.FormatSpanishDateTime) funcione sin depender de que la imagen
	// Docker traiga el paquete "tzdata" del sistema operativo — Alpine (la
	// imagen base de este proyecto) no lo trae por defecto, y sin esto
	// LoadLocation fallaría en silencio y los correos/WhatsApp mostrarían
	// la hora en UTC en vez de la hora local del consultorio.
	_ "time/tzdata"
)

func main() {
	// 1. Cargar variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró archivo .env, usando variables de entorno del sistema")
	}

	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("Error crítico: la variable de entorno JWT_SECRET no está definida")
	}

	observability.InitSentry()
	defer sentrygo.Flush(2 * time.Second)

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
	db.AutoMigrate(&models.Doctor{}, &models.Patient{}, &models.MedicalHistory{}, &models.Appointment{}, &models.MedicalDocument{}, &models.DoctorTemplate{}, &models.Staff{}, &models.DoctorStaff{}, &models.SuperAdmin{}, &models.DoctorSchedule{}, &models.DoctorGalleryImage{}, &models.Review{})

	// Migración de compatibilidad: Staff dejó de pertenecer a un solo
	// doctor (columnas doctor_id/active) y ahora se vincula a uno o más
	// doctores por medio de doctor_staff (ver models.DoctorStaff), para
	// soportar que una misma cuenta de personal administre varios
	// consultorios/doctores con un solo login. Si el despliegue anterior
	// todavía tiene la columna vieja, se respalda el vínculo existente en
	// doctor_staff antes de eliminarla — así el personal ya invitado no
	// pierde acceso a su doctor actual.
	if db.Migrator().HasColumn(&models.Staff{}, "doctor_id") {
		if err := db.Exec(`
			INSERT INTO doctor_staff (created_at, doctor_id, staff_id, active)
			SELECT NOW(), doctor_id, id, COALESCE(active, true) FROM staffs WHERE doctor_id IS NOT NULL
			ON CONFLICT DO NOTHING
		`).Error; err != nil {
			log.Println("Aviso: no se pudo respaldar staffs.doctor_id en doctor_staff:", err)
		}
		if err := db.Migrator().DropColumn(&models.Staff{}, "doctor_id"); err != nil {
			log.Println("Aviso: no se pudo eliminar la columna vieja staffs.doctor_id:", err)
		}
	}
	if db.Migrator().HasColumn(&models.Staff{}, "active") {
		if err := db.Migrator().DropColumn(&models.Staff{}, "active"); err != nil {
			log.Println("Aviso: no se pudo eliminar la columna vieja staffs.active:", err)
		}
	}

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
	database.SeedSuperAdmin(db)

	// Cliente de WhatsApp (Twilio): solo se construye si las tres
	// variables están configuradas. Sin eso, los workers de recordatorio
	// simplemente no mandan WhatsApp (el correo sigue funcionando igual).
	whatsappConfig := whatsapp.LoadConfigFromEnv()
	var whatsappClient whatsapp.Client
	if whatsappConfig.IsConfigured() {
		whatsappClient = whatsapp.NewClient(whatsappConfig)
	}

	whatsappTemplates := whatsapp.LoadTemplatesFromEnv()

	workers.StartNightClosureWorker(db)
	workers.StartAppointmentReminderWorker(db, auth.SendEmail, whatsappClient, whatsappTemplates)
	// Recordatorio al doctor: por correo, no WhatsApp — el doctor ya tiene
	// que entrar a la app para iniciar la consulta, ver el comentario en
	// workers.StartDoctorReminderWorker.
	workers.StartDoctorReminderWorker(db, auth.SendEmail)

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
