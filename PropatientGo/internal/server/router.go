package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"propatient-api/internal/auth"
	"propatient-api/internal/billing"
	"propatient-api/internal/geocoding"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/handlers"
	"propatient-api/internal/storage"
	"propatient-api/internal/whatsapp"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NewRouter arma el *gin.Engine completo de la API: CORS, archivos estáticos,
// health check y todas las rutas públicas/protegidas. Extraído de main.go
// para poder levantar la app real (con su middleware real) en los tests de
// integración, sin duplicar el registro de rutas.
func NewRouter(db *gorm.DB) *gin.Engine {
	// Cliente de Google Calendar real, construido desde variables de
	// entorno. Solo se usa de verdad si el doctor conectó su cuenta
	// (refresh token guardado) y el servidor tiene GOOGLE_CLIENT_SECRET
	// configurado; sin eso, todas las llamadas relacionadas quedan en
	// no-op — no requiere credenciales para que el resto de la app funcione.
	calendarConfig := googlecalendar.LoadConfigFromEnv()

	// Cliente de archivos: S3 si AWS_S3_BUCKET/AWS_REGION están
	// configurados, si no cae de vuelta al disco local (mismo
	// comportamiento que tenía la app antes de esta integración). Un error
	// aquí solo puede venir de credenciales de AWS mal formadas — no debe
	// tumbar el servidor entero, así que se degrada a disco local.
	storageConfig := storage.LoadConfigFromEnv()
	storageClient, err := storage.NewClient(context.Background(), storageConfig)
	if err != nil {
		log.Printf("⚠️ No se pudo inicializar el almacenamiento en S3, usando disco local: %v", err)
		storageClient, _ = storage.NewClient(context.Background(), storage.Config{})
	}

	// Cliente de Stripe: solo se construye si STRIPE_SECRET_KEY y
	// STRIPE_PRICE_ID están configuradas. Sin eso, las rutas de facturación
	// devuelven 503 y el resto de la app funciona igual (RequireActiveSubscription
	// solo bloquea si un doctor concreto ya no tiene prueba/suscripción vigente).
	billingConfig := billing.LoadConfigFromEnv()
	var billingClient billing.Client
	if billingConfig.IsConfigured() {
		billingClient = billing.NewClient(billingConfig)
	}

	// Geocodificación (Nominatim/OpenStreetMap): sin API key, siempre
	// disponible. Solo se llama cuando un doctor guarda su perfil con el
	// listado público activo.
	geoClient := geocoding.NewClient()

	// Cliente de WhatsApp (Twilio): solo se construye si las tres variables
	// están configuradas. Sin eso, los avisos por WhatsApp simplemente no
	// se mandan (mejor esfuerzo, igual que el correo) y el resto de la app
	// funciona igual.
	whatsappConfig := whatsapp.LoadConfigFromEnv()
	var whatsappClient whatsapp.Client
	if whatsappConfig.IsConfigured() {
		whatsappClient = whatsapp.NewClient(whatsappConfig)
	}

	return NewRouterWithDeps(db, calendarConfig, googlecalendar.NewClient(calendarConfig), storageClient, billingConfig, billingClient, geoClient, whatsappClient)
}

// NewRouterWithDeps es la variante inyectable de NewRouter: permite a los
// tests de integración pasar un googlecalendar.Client y un storage.Client
// simulados, sin hacer llamadas reales a Google ni a AWS.
func NewRouterWithDeps(db *gorm.DB, calendarConfig googlecalendar.Config, calendarClient googlecalendar.Client, storageClient storage.Client, billingConfig billing.Config, billingClient billing.Client, geoClient geocoding.Client, whatsappClient whatsapp.Client) *gin.Engine {
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
			authRoutes.POST("/staff-login", handlers.StaffLoginHandler(db))
			authRoutes.GET("/staff-invite/:token", handlers.GetStaffInvite(db))
			authRoutes.POST("/staff-invite/:token", handlers.AcceptStaffInvite(db))
			// Callback de Google Calendar: lo llama el navegador tras el
			// consentimiento (redirect de Google), sin header Authorization,
			// así que va fuera del grupo protegido. La identidad del doctor
			// viaja firmada en el parámetro "state".
			authRoutes.GET("/google-calendar/callback", handlers.GoogleCalendarCallback(db, calendarConfig))
		}

		// Webhook de Stripe: lo llama Stripe directamente (sin Authorization,
		// sin sesión de usuario). La firma Stripe-Signature es lo único que
		// autentica la petición, así que va fuera de /auth y del grupo protegido.
		api.POST("/billing/webhook", handlers.StripeWebhook(db, billingConfig))

		// Directorio público (landing page): sin autenticación, para que
		// cualquier visitante pueda ver doctores y agendar una solicitud de
		// cita. Nunca expone historial clínico ni datos de otros pacientes.
		public := api.Group("/public")
		{
			public.GET("/doctors", handlers.GetPublicDoctors(db, storageClient))
			public.GET("/doctors/:slug", handlers.GetPublicDoctorBySlug(db, storageClient))
			public.POST("/appointments", handlers.CreatePublicAppointment(db, whatsappClient))
		}

		// --- RUTAS PROTEGIDAS ---
		// Usamos un grupo vacío "" para que las rutas cuelguen de /api/ directamente
		protected := api.Group("")
		protected.Use(auth.AuthorizeJWT()) // Este middleware ya tiene el fix del IF OPTIONS
		{
			// Facturación: fuera de RequireActiveSubscription a propósito — un
			// doctor con la prueba vencida tiene que poder llegar aquí para
			// pagar. Solo el doctor la gestiona, nunca el personal.
			billingRoutes := protected.Group("/billing")
			billingRoutes.Use(auth.RequireDoctorRole())
			{
				billingRoutes.GET("/status", handlers.GetBillingStatus(db))
				billingRoutes.POST("/checkout", handlers.CreateCheckoutSession(db, billingClient, billingConfig))
				billingRoutes.POST("/portal", handlers.CreatePortalSession(db, billingClient))
			}

			// Todo lo demás requiere prueba vigente o suscripción activa. El
			// personal comparte el estado de suscripción del doctor que lo
			// invitó (mismo doctorID en el token), así que si el doctor no
			// paga, el personal también queda bloqueado.
			gated := protected.Group("")
			gated.Use(billing.RequireActiveSubscription(db))
			{
				dashboard := gated.Group("/dashboard")
				{
					dashboard.GET("/summary", handlers.GetTodaySummary(db))
					dashboard.GET("/upcoming", handlers.GetUpcomingAppointments(db))
					dashboard.GET("/stats", handlers.GetConsultorioStats(db))
					dashboard.GET("/follow-ups", handlers.GetFollowUps(db))
				}

				// Datos de la propia cuenta del doctor (RFC/CURP/cédula/INE):
				// nunca disponible para el personal.
				users := gated.Group("/user")
				users.Use(auth.RequireDoctorRole())
				{
					users.POST("/update-profile", auth.UpdateProfileHandler(db, geoClient))
					users.POST("/update-license", auth.UpdateLicenseHandler(db))
					users.POST("/update-license-full", auth.UpdateLicenseFullHandler(db, storageClient))
				}

				patients := gated.Group("/patients")
				{
					// Agenda y datos generales: disponible también para personal.
					patients.GET("", handlers.GetPatients(db))
					patients.POST("", handlers.CreatePatient(db))
					patients.GET("/search", handlers.SearchPatients(db))
					patients.GET("/:id", handlers.GetPatientById(db))
					patients.GET("/:id/stats", handlers.GetPatientStats(db))
					patients.PUT("/:id", handlers.UpdatePatient(db))
					patients.DELETE("/:id", handlers.RemovePatientFromDoctor(db))
					patients.POST("/:id/link", handlers.LinkExistingPatient(db))
					// Historial clínico: solo el doctor.
					patients.GET("/:id/history", auth.RequireDoctorRole(), handlers.GetPatientMedicalHistory(db, storageClient))
					patients.PUT("/:id/medical-history", auth.RequireDoctorRole(), handlers.UpdateMedicalHistory(db))
				}

				appointments := gated.Group("/appointments")
				{
					// Importante: Usar "" en lugar de "/" para evitar redirecciones 301/307
					// que el navegador a veces bloquea en CORS.
					// Agenda: disponible también para personal (agendar/cancelar,
					// sin ver el contenido clínico de la consulta).
					appointments.GET("", handlers.GetAppointments(db, storageClient))
					appointments.POST("", handlers.CreateAppointment(db, calendarClient))
					appointments.PUT("/:id/cancel", handlers.CancelAppointment(db, calendarClient, whatsappClient))
					appointments.PUT("/:id/confirm", handlers.ConfirmAppointment(db, calendarClient, whatsappClient))
					// Contenido clínico de la consulta: solo el doctor.
					appointments.GET("/:id", auth.RequireDoctorRole(), handlers.GetAppointmentDetail(db, storageClient))
					appointments.PUT("/:id", auth.RequireDoctorRole(), handlers.UpdateAppointment(db, calendarClient, whatsappClient))
					appointments.POST("/:id/upload-document", auth.RequireDoctorRole(), handlers.UploadDocuments(db, storageClient))
					appointments.PUT("/:id/documents/:docId", auth.RequireDoctorRole(), handlers.UpdateAppointmentDocument(db))
					appointments.POST("/:id/save-recipe-pdf", auth.RequireDoctorRole(), handlers.SaveRecipePDF(db, storageClient))
				}

				// Perfil, plantillas y Google Calendar del doctor: nunca para el personal.
				doctorRoutes := gated.Group("/doctor")
				doctorRoutes.Use(auth.RequireDoctorRole())
				{
					doctorRoutes.GET("/me", handlers.GetCurrentDoctor(db, storageClient))
					doctorRoutes.PUT("/me", handlers.UpdateCurrentDoctor(db, storageClient, geoClient))
					// Ubica una dirección en el mapa sin guardarla: la usa el mapa
					// interactivo del registro y del perfil para centrar el pin.
					doctorRoutes.GET("/geocode", handlers.PreviewGeocode(geoClient))
					doctorRoutes.GET("/template", handlers.GetDoctorTemplate(db))
					doctorRoutes.POST("/template", handlers.SaveDoctorTemplate(db))
					doctorRoutes.GET("/google-calendar/connect", handlers.ConnectGoogleCalendar(calendarConfig))
					doctorRoutes.POST("/google-calendar/disconnect", handlers.DisconnectGoogleCalendar(db))
				}

				// Alta/gestión de cuentas de personal: solo el doctor.
				staffRoutes := gated.Group("/staff")
				staffRoutes.Use(auth.RequireDoctorRole())
				{
					staffRoutes.GET("", handlers.ListStaff(db))
					staffRoutes.POST("", handlers.InviteStaff(db))
					staffRoutes.PUT("/:id/active", handlers.ToggleStaffActive(db))
					staffRoutes.DELETE("/:id", handlers.DeleteStaff(db))
				}

				utils := gated.Group("/utils")
				{
					utils.GET("/specialties", handlers.GetSpecialties)
				}
			}
		}
	}

	return r
}
