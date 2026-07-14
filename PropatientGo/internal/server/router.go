package server

import (
	"net/http"
	"os"
	"strings"

	"propatient-api/internal/auth"
	"propatient-api/internal/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NewRouter arma el *gin.Engine completo de la API: CORS, archivos estáticos,
// health check y todas las rutas públicas/protegidas. Extraído de main.go
// para poder levantar la app real (con su middleware real) en los tests de
// integración, sin duplicar el registro de rutas.
func NewRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()
	r.MaxMultipartMemory = 8 << 20 // 8 MiB por request de carga de archivos

	// El origen del frontend se toma de FRONTEND_URL (soporta varios separados
	// por coma, útil para tener local + producción a la vez). Si no está
	// definida, usa el dev local.
	allowedOrigins := []string{"http://localhost:5173"}
	if frontendURL := os.Getenv("FRONTEND_URL"); frontendURL != "" {
		var parsed []string
		for _, origin := range strings.Split(frontendURL, ",") {
			origin = strings.TrimSpace(origin)
			origin = strings.TrimSuffix(origin, "/")
			if origin == "" {
				continue // ignora comas sobrantes, ej. "https://foo.com,"
			}
			// gin-contrib/cors hace panic() si un origen no es "*" y no
			// incluye esquema. Si alguien pega solo el dominio (sin
			// "https://") en el dashboard de Render, lo normalizamos en
			// vez de tumbar el servidor entero al arrancar.
			if origin != "*" && !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
				origin = "https://" + origin
			}
			parsed = append(parsed, origin)
		}
		if len(parsed) > 0 {
			allowedOrigins = parsed
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins, // URL EXACTA, NO "*"
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	r.RedirectTrailingSlash = true
	r.RedirectFixedPath = false

	// Registrado DESPUÉS de cors.New(): si se registra antes, gin no incluye
	// el middleware de CORS en la cadena de esta ruta y las imágenes servidas
	// aquí (logo/avatar del doctor) quedan sin cabecera
	// Access-Control-Allow-Origin. Eso rompía la carga de la imagen vía
	// <img crossOrigin="anonymous"> que usa el frontend para incrustar el
	// logo en el PDF de la receta (fallaba en silencio y caía al texto de
	// respaldo "MÉDICO GENERAL").
	r.Static("/uploads", "./uploads")

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			dbStatus := "conectada"
			httpStatus := http.StatusOK
			if sqlDB, err := db.DB(); err != nil || sqlDB.Ping() != nil {
				dbStatus = "desconectada"
				httpStatus = http.StatusServiceUnavailable
			}
			c.JSON(httpStatus, gin.H{
				"status":  "ok",
				"message": "¡Hola! El backend en Docker está vivo 🚀",
				"db":      dbStatus,
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
			dashboard := protected.Group("/dashboard")
			{
				dashboard.GET("/summary", handlers.GetTodaySummary(db))
				dashboard.GET("/upcoming", handlers.GetUpcomingAppointments(db))
				dashboard.GET("/stats", handlers.GetConsultorioStats(db))
			}

			users := protected.Group("/user")
			{
				users.POST("/update-profile", auth.UpdateProfileHandler(db))
				users.POST("/update-license", auth.UpdateLicenseHandler(db))
				users.POST("/update-license-full", auth.UpdateLicenseFullHandler(db))
			}

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
				patients.DELETE("/:id", handlers.RemovePatientFromDoctor(db))
			}

			appointments := protected.Group("/appointments")
			{
				// Importante: Usar "" en lugar de "/" para evitar redirecciones 301/307
				// que el navegador a veces bloquea en CORS.
				appointments.GET("", handlers.GetAppointments(db))
				appointments.POST("", handlers.CreateAppointment(db))
				appointments.GET("/:id", handlers.GetAppointmentDetail(db))
				appointments.PUT("/:id", handlers.UpdateAppointment(db))
				appointments.PUT("/:id/cancel", handlers.CancelAppointment(db))
				appointments.POST("/:id/upload-document", handlers.UploadDocuments(db))
				appointments.PUT("/:id/documents/:docId", handlers.UpdateAppointmentDocument(db))
				appointments.POST("/:id/save-recipe-pdf", handlers.SaveRecipePDF(db))
			}

			doctorRoutes := protected.Group("/doctor")
			{
				doctorRoutes.GET("/me", handlers.GetCurrentDoctor(db))
				doctorRoutes.PUT("/me", handlers.UpdateCurrentDoctor(db))
				doctorRoutes.GET("/template", handlers.GetDoctorTemplate(db))
				doctorRoutes.POST("/template", handlers.SaveDoctorTemplate(db))
			}

			utils := protected.Group("/utils")
			{
				utils.GET("/specialties", handlers.GetSpecialties)
			}
		}
	}

	return r
}
