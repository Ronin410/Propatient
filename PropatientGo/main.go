package main

import (
	"log"
	"os"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/database"
	"propatient-api/internal/handlers"
	"propatient-api/internal/models"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1. Cargar variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró archivo .env, usando variables de entorno del sistema")
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
	db.AutoMigrate(&models.Doctor{}, &models.Patient{}, &models.MedicalHistory{}, &models.Appointment{}, &models.MedicalDocument{})
	database.SeedDatabase(db)

	// 4. Configuración del Router
	r := gin.Default()

	// 5. CORS - DEBE ir antes de cualquier ruta
	// Esta configuración permite que el navegador valide los permisos antes de enviar el Token
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"}, // URL EXACTA, NO "*"
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	r.RedirectTrailingSlash = true
	r.RedirectFixedPath = false

	// 6. Grupo de Rutas API
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":  "ok",
				"message": "¡Hola! El backend en Docker está vivo 🚀",
				"db":      "conectada",
			})
		})

		// --- RUTAS PÚBLICAS ---
		authRoutes := api.Group("/auth")
		{
			authRoutes.POST("/login", auth.LoginHandler(db))
			authRoutes.POST("/register", auth.RegisterDoctor(db))
			authRoutes.POST("/google-login", auth.GoogleLoginHandler(db))
		}

		// --- RUTAS PROTEGIDAS ---
		// Usamos un grupo vacío "" para que las rutas cuelguen de /api/ directamente
		protected := api.Group("")
		protected.Use(auth.AuthorizeJWT()) // Este middleware ya tiene el fix del IF OPTIONS
		{
			// Dashboard
			dashboard := protected.Group("/dashboard")
			{
				dashboard.GET("/summary", handlers.GetTodaySummary(db))
				dashboard.GET("/upcoming", handlers.GetUpcomingAppointments(db))
			}

			users := protected.Group("/user")
			{
				users.POST("/update-profile", auth.UpdateProfileHandler(db))
				users.POST("/update-license", auth.UpdateLicenseHandler(db))
				users.POST("/update-license-full", auth.UpdateLicenseFullHandler(db))
				// userRoutes.POST("/verify-cedula", auth.VerifyCedulaHandler(db))
			}

			// Patients
			patients := protected.Group("/patients")
			{
				patients.GET("", handlers.GetPatients(db))
				patients.POST("", handlers.CreatePatient(db))
				patients.GET("/search", handlers.SearchPatients(db))
				patients.GET("/:id", handlers.GetPatientById(db))
				patients.GET("/:id/history", handlers.GetPatientMedicalHistory(db))
				patients.GET("/:id/stats", handlers.GetPatientStats(db))
				patients.PUT("/:id", handlers.UpdatePatient(db))
				patients.PUT("/:id/medical-history", handlers.UpdateMedicalHistory(db))
			}

			// Appointments
			appointments := protected.Group("/appointments")
			{
				// Importante: Usar "" en lugar de "/" para evitar redirecciones 301/307
				// que el navegador a veces bloquea en CORS.
				appointments.GET("", handlers.GetAppointments(db))
				appointments.POST("", handlers.CreateAppointment(db))
				appointments.GET("/:id", handlers.GetAppointmentDetail(db))
				appointments.PUT("/:id", handlers.UpdateAppointment(db))
			}

			// Doctor
			doctorRoutes := protected.Group("/doctor")
			{
				doctorRoutes.GET("/me", handlers.GetCurrentDoctor(db))
				doctorRoutes.PUT("/me", handlers.UpdateCurrentDoctor(db))
			}

			// Utils
			utils := protected.Group("/utils")
			{
				utils.GET("/specialties", handlers.GetSpecialties)
			}
		}
	}

	// 7. Lanzar Servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "8095"
	}

	log.Printf("🚀 ProPatient API (Go) corriendo en puerto %s", port)
	r.Run(":" + port)
}
