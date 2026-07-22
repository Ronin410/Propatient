package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/models"
	"propatient-api/internal/recaptcha"
	"propatient-api/internal/storage"
	"propatient-api/internal/webpush"
	"propatient-api/internal/whatsapp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PublicDoctorSummary es el subconjunto de campos de un doctor que se
// expone en el directorio público — nunca el modelo Doctor completo (aunque
// hoy ya protege lo sensible con json:"-", usar un struct dedicado evita
// que un campo nuevo se filtre por accidente el día que alguien lo agregue
// al modelo).
type PublicDoctorSummary struct {
	ID               uint     `json:"id"`
	FullName         string   `json:"fullName"`
	MedicalSpecialty string   `json:"medicalSpecialty"`
	PublicBio        string   `json:"publicBio"`
	AvatarUrl        string   `json:"avatarUrl"`
	Address          string   `json:"address"`
	Phone            string   `json:"phone"`
	Latitude         *float64 `json:"latitude"`
	Longitude        *float64 `json:"longitude"`
	PublicSlug       string   `json:"publicSlug"`
	// Horario laboral configurado (ver DoctorSchedule) — nil si el doctor
	// nunca lo configuró, para que el frontend sepa que no hay ninguna
	// restricción que mostrarle al paciente antes de agendar.
	Schedule *weekSchedule `json:"schedule"`

	// Redes sociales / sitio propio, solo si el doctor las llenó.
	FacebookUrl  string `json:"facebookUrl,omitempty"`
	InstagramUrl string `json:"instagramUrl,omitempty"`
	LinkedinUrl  string `json:"linkedinUrl,omitempty"`
	TwitterUrl   string `json:"twitterUrl,omitempty"`
	TiktokUrl    string `json:"tiktokUrl,omitempty"`
	YoutubeUrl   string `json:"youtubeUrl,omitempty"`
	WebsiteUrl   string `json:"websiteUrl,omitempty"`

	// Galería de fotos y reseñas aprobadas: solo se llenan en el perfil
	// individual (ver GetPublicDoctorBySlug), no en el listado del
	// directorio — evitan consultas extra por doctor en un listado que
	// puede tener muchos.
	GalleryImages []models.DoctorGalleryImage `json:"galleryImages,omitempty"`
	Reviews       []publicReviewSummary       `json:"reviews,omitempty"`
	ReviewsAvg    *float64                    `json:"reviewsAverage,omitempty"`
}

// publicReviewSummary es lo único de una reseña que se expone en el
// perfil público — nunca el ID/token/datos del paciente que dejó la
// reseña, solo su primer nombre para que se sienta real sin identificarlo
// del todo.
type publicReviewSummary struct {
	PatientFirstName string    `json:"patientFirstName"`
	Rating           int       `json:"rating"`
	Comment          string    `json:"comment"`
	SubmittedAt      time.Time `json:"submittedAt"`
}

func toPublicDoctorSummary(d models.Doctor) PublicDoctorSummary {
	return PublicDoctorSummary{
		ID:               d.ID,
		FullName:         d.FullName,
		MedicalSpecialty: d.MedicalSpecialty,
		PublicBio:        d.PublicBio,
		AvatarUrl:        d.AvatarUrl,
		Address:          d.Address,
		Phone:            d.Phone,
		Latitude:         d.Latitude,
		Longitude:        d.Longitude,
		PublicSlug:       d.PublicSlug,
		FacebookUrl:      d.FacebookUrl,
		InstagramUrl:     d.InstagramUrl,
		LinkedinUrl:      d.LinkedinUrl,
		TwitterUrl:       d.TwitterUrl,
		TiktokUrl:        d.TiktokUrl,
		YoutubeUrl:       d.YoutubeUrl,
		WebsiteUrl:       d.WebsiteUrl,
	}
}

// isDoctorAcceptingPublicBookings replica la misma regla de
// billing.RequireActiveSubscription (prueba vigente o suscripción activa),
// para no exponer en el directorio ni dejar agendar con un consultorio que
// ya no tiene acceso al sistema.
func isDoctorAcceptingPublicBookings(d models.Doctor) bool {
	if d.SubscriptionStatus == "active" {
		return true
	}
	return d.SubscriptionStatus == "trialing" && d.TrialEndsAt != nil && time.Now().UTC().Before(*d.TrialEndsAt)
}

// GetPublicDoctors devuelve el directorio de doctores que activaron su
// listado público. Sin autenticación.
func GetPublicDoctors(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var doctors []models.Doctor
		if err := db.Where("public_listed = ?", true).Find(&doctors).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener el directorio de doctores"})
			return
		}

		summaries := make([]PublicDoctorSummary, 0, len(doctors))
		for i := range doctors {
			if !isDoctorAcceptingPublicBookings(doctors[i]) {
				continue
			}
			presignDoctorFiles(c.Request.Context(), storageClient, &doctors[i])
			summaries = append(summaries, toPublicDoctorSummary(doctors[i]))
		}

		c.JSON(http.StatusOK, summaries)
	}
}

// GetPublicDoctorBySlug devuelve el perfil público de un solo doctor, para
// la página /dr/:slug. Sin autenticación.
func GetPublicDoctorBySlug(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")

		var doctor models.Doctor
		if err := db.Where("public_slug = ? AND public_listed = ?", slug, true).First(&doctor).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if !isDoctorAcceptingPublicBookings(doctor) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		presignDoctorFiles(c.Request.Context(), storageClient, &doctor)
		summary := toPublicDoctorSummary(doctor)

		// El horario solo se manda en el perfil individual (donde vive el
		// formulario de agendar), no en el directorio — evita una consulta
		// extra por doctor en un listado que puede tener muchos.
		var schedule models.DoctorSchedule
		if err := db.Where("doctor_id = ?", doctor.ID).First(&schedule).Error; err == nil {
			var week weekSchedule
			if json.Unmarshal(schedule.Days, &week) == nil {
				summary.Schedule = &week
			}
		}

		// Galería de fotos, igual que el horario: solo en el perfil individual.
		var images []models.DoctorGalleryImage
		if err := db.Where("doctor_id = ?", doctor.ID).Order("created_at ASC").Find(&images).Error; err == nil {
			presignGalleryImages(c.Request.Context(), storageClient, images)
			summary.GalleryImages = images
		}

		// Reseñas aprobadas por el doctor, más recientes primero, con su
		// promedio — un paciente nunca ve las que siguen pendientes de
		// revisión.
		var reviews []struct {
			models.Review
			PatientFirstName string `json:"patientFirstName"`
		}
		if err := db.Table("reviews").
			Select("reviews.*, patients.first_name AS patient_first_name").
			Joins("JOIN patients ON patients.id = reviews.patient_id").
			Where("reviews.doctor_id = ? AND reviews.approved = ?", doctor.ID, true).
			Order("reviews.submitted_at DESC").
			Scan(&reviews).Error; err == nil && len(reviews) > 0 {
			ratingSum := 0
			publicReviews := make([]publicReviewSummary, 0, len(reviews))
			for _, r := range reviews {
				ratingSum += r.Rating
				submittedAt := time.Time{}
				if r.SubmittedAt != nil {
					submittedAt = *r.SubmittedAt
				}
				publicReviews = append(publicReviews, publicReviewSummary{
					PatientFirstName: r.PatientFirstName,
					Rating:           r.Rating,
					Comment:          r.Comment,
					SubmittedAt:      submittedAt,
				})
			}
			summary.Reviews = publicReviews
			avg := float64(ratingSum) / float64(len(reviews))
			summary.ReviewsAvg = &avg
		}

		c.JSON(http.StatusOK, summary)
	}
}

type publicAppointmentRequest struct {
	DoctorID            uint      `json:"doctorId" binding:"required"`
	AppointmentDateTime time.Time `json:"appointmentDateTime" binding:"required"`
	Reason              string    `json:"reason"`
	PatientFirstName    string    `json:"patientFirstName" binding:"required"`
	PatientLastName     string    `json:"patientLastName" binding:"required"`
	PatientPhone        string    `json:"patientPhone" binding:"required"`
	PatientEmail        string    `json:"patientEmail" binding:"required,email"`
	// El paciente debe aceptar explícitamente el tratamiento de sus datos de
	// salud (dato personal sensible bajo la LFPDPPP) antes de poder agendar
	// sin cuenta — se valida a mano abajo (no con `binding:"required"`) para
	// poder devolver un mensaje en español en vez del error genérico de
	// validación de Gin.
	DataConsent bool `json:"dataConsent"`
	// Si el paciente es menor de edad, el teléfono/correo de arriba son los
	// del padre/madre o tutor (no puede tener los suyos propios) — el
	// frontend cambia el texto del checkbox de consentimiento para que sea
	// una declaración explícita de patria potestad/tutela en ese caso (ver
	// PublicDoctorProfile.tsx). Se guarda en el expediente del paciente
	// (Patient.IsMinor) para que quede documentado quién autorizó el
	// tratamiento de sus datos.
	IsMinorPatient bool `json:"isMinorPatient"`
	// Token de reCAPTCHA v3 generado por el frontend. Opcional a nivel de
	// request: si el backend no tiene RECAPTCHA_SECRET_KEY configurada,
	// recaptcha.Verify no exige nada (ver ese paquete).
	RecaptchaToken string `json:"recaptchaToken"`
}

// CreatePublicAppointment permite a cualquier persona (sin cuenta) solicitar
// una cita con un doctor del directorio público. La cita nace en estado
// PENDING_CONFIRMATION: no aparece como una cita real hasta que el doctor
// (o su personal) la confirma con ConfirmAppointment — así una solicitud
// falsa o spam nunca reserva un horario de verdad sin que alguien la revise.
func CreatePublicAppointment(db *gorm.DB, waClient whatsapp.Client, waTemplates whatsapp.Templates, pushClient webpush.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req publicAppointmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !req.DataConsent {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Debes aceptar el tratamiento de tus datos de salud para poder agendar la cita."})
			return
		}
		if err := recaptcha.Verify(c.Request.Context(), req.RecaptchaToken, "public_appointment"); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var doctor models.Doctor
		if err := db.Where("id = ? AND public_listed = ?", req.DoctorID, true).First(&doctor).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Este doctor no está disponible para agendar citas en línea"})
			return
		}
		if !isDoctorAcceptingPublicBookings(doctor) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Este consultorio no está aceptando citas en este momento"})
			return
		}
		if err := ValidateAppointmentAgainstSchedule(db, doctor.ID, req.AppointmentDateTime); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		patient, alreadyLinked, err := findOrCreatePublicBookingPatient(db, doctor, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo registrar tus datos"})
			return
		}

		if !alreadyLinked {
			if err := db.Model(&doctor).Association("Patients").Append(patient); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo vincular tu solicitud con el consultorio"})
				return
			}
		}

		appointment := models.Appointment{
			PatientID:           patient.ID,
			DoctorID:            doctor.ID,
			AppointmentDateTime: req.AppointmentDateTime,
			Reason:              req.Reason,
			Status:              "PENDING_CONFIRMATION",
			RegistrationStatus:  "PENDING_RECORD",
		}
		if err := db.Create(&appointment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo registrar tu solicitud de cita"})
			return
		}

		// Evidencia del consentimiento (LFPDPPP, dato de salud sensible):
		// req.DataConsent ya se validó como obligatorio arriba, pero antes
		// de esto la aceptación en sí no quedaba registrada en ningún lado,
		// solo el hecho de que la cita existiera. No se aborta la petición
		// si esto falla (la cita ya se guardó y el paciente no tiene por
		// qué ver un error por esto) — solo se deja constancia en el log.
		if err := db.Create(&models.ConsentRecord{
			PatientID:          patient.ID,
			AppointmentID:      appointment.ID,
			DoctorID:           doctor.ID,
			AcceptedAsGuardian: req.IsMinorPatient,
			NoticeVersion:      models.CurrentLegalNoticeVersion,
			IPAddress:          c.ClientIP(),
		}).Error; err != nil {
			log.Printf("⚠️ No se pudo guardar la evidencia de consentimiento de la cita %d: %v", appointment.ID, err)
		}

		// Correos y WhatsApp de aviso: en segundo plano, nunca deben poder
		// retrasar ni tumbar la respuesta al paciente — la solicitud ya se
		// guardó. Van en su propia goroutine (no en la petición HTTP) porque
		// SMTP/Twilio pueden tardar varios segundos o quedarse colgados (ej.
		// si el hosting bloquea el puerto SMTP saliente), y eso alargaría
		// esta petición pública hasta el punto de que el navegador la dé por
		// fallida aunque la cita sí se haya guardado. Se usa
		// context.Background() en vez del contexto de la petición porque
		// ese se cancela en cuanto el handler responde, lo que cortaría a
		// medias la llamada a Twilio. El recover() evita que un pánico
		// inesperado aquí tumbe todo el proceso (una goroutine sin
		// recuperar sí puede hacerlo, a diferencia del resto del handler,
		// que ya está protegido por el middleware Recovery de Gin).
		go func(doctor models.Doctor, patient models.Patient, appointment models.Appointment) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("⚠️ Pánico recuperado al mandar avisos de la solicitud de cita %d: %v", appointment.ID, r)
				}
			}()
			// El WhatsApp al paciente va primero: si se logra entregar, el
			// correo al paciente (mismo contenido) se salta para no
			// duplicarlo — ver sendPublicBookingWhatsApp/
			// sendPublicBookingEmails. El correo al DOCTOR no se toca,
			// siempre se manda por los dos canales.
			patientNotifiedByWhatsApp := sendPublicBookingWhatsApp(context.Background(), waClient, waTemplates, doctor, patient, appointment)
			sendPublicBookingEmails(doctor, patient, appointment, patientNotifiedByWhatsApp)
			sendPublicBookingPush(context.Background(), db, pushClient, doctor, patient, appointment)
		}(doctor, *patient, appointment)

		c.JSON(http.StatusCreated, gin.H{
			"message": "Tu solicitud de cita fue enviada. El consultorio la confirmará pronto por teléfono o correo.",
		})
	}
}

// sendPublicBookingEmails avisa por correo tanto al paciente (confirmación
// de que su solicitud se recibió, todavía sin confirmar) como al doctor
// (aviso de que tiene una solicitud nueva por revisar en su panel).
// skipPatientEmail es true cuando el mismo aviso ya se le entregó al
// paciente por WhatsApp (ver sendPublicBookingWhatsApp) — evita mandarle
// el mismo aviso dos veces por canales distintos. El correo al doctor
// nunca se salta, sin importar si su WhatsApp se envió o no.
func sendPublicBookingEmails(doctor models.Doctor, patient models.Patient, appointment models.Appointment, skipPatientEmail bool) {
	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)

	if patient.Email != "" && !skipPatientEmail {
		subject := "Recibimos tu solicitud de cita con Dr(a). " + doctor.FullName
		body := fmt.Sprintf(
			`<p>Hola %s,</p>
			<p>Recibimos tu solicitud de cita con <strong>Dr(a). %s</strong> para el <strong>%s</strong>.</p>
			<p>El consultorio revisará tu solicitud y te confirmará pronto por correo o teléfono. Tu cita <strong>todavía no está agendada</strong> hasta que la confirmen.</p>
			<p>— ProPatient</p>`,
			patient.FirstName, doctor.FullName, when,
		)
		if err := auth.SendEmail(patient.Email, subject, body); err != nil {
			log.Printf("⚠️ No se pudo enviar el correo de confirmación al paciente (cita %d): %v", appointment.ID, err)
		}
	}

	if doctor.Email != "" {
		reasonLine := ""
		if appointment.Reason != "" {
			reasonLine = "<br>Motivo: " + appointment.Reason
		}
		subject := "Nueva solicitud de cita por confirmar"
		body := fmt.Sprintf(
			`<p>Tienes una nueva solicitud de cita en línea:</p>
			<p><strong>%s %s</strong><br>
			Fecha solicitada: %s<br>
			Teléfono: %s<br>
			Correo: %s%s</p>
			<p>Revísala y confírmala desde tu panel de ProPatient, en "Solicitudes de Cita Nuevas".</p>`,
			patient.FirstName, patient.LastName, when, patient.Phone, patient.Email, reasonLine,
		)
		if err := auth.SendEmail(doctor.Email, subject, body); err != nil {
			log.Printf("⚠️ No se pudo enviar el aviso de nueva solicitud al doctor %d: %v", doctor.ID, err)
		}
	}
}

// sendPublicBookingWhatsApp es el equivalente por WhatsApp de
// sendPublicBookingEmails: mismo contenido, mismo criterio de "mejor
// esfuerzo" (no interrumpe la reserva si Twilio no está configurado o
// falla). waClient es nil si Twilio no está configurado (mismo patrón que
// calClient en googlecalendar) — en ese caso no hace nada. Devuelve true si
// el paciente (no el doctor) quedó notificado por este canal, para que el
// llamador decida si todavía hace falta mandarle el correo equivalente.
func sendPublicBookingWhatsApp(ctx context.Context, waClient whatsapp.Client, waTemplates whatsapp.Templates, doctor models.Doctor, patient models.Patient, appointment models.Appointment) bool {
	if waClient == nil {
		return false
	}
	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)
	patientNotified := false

	if patient.Phone != "" {
		body := fmt.Sprintf(
			"Hola %s, recibimos tu solicitud de cita con Dr(a). %s para el %s. El consultorio la revisará y te confirmará pronto — tu cita todavía no está agendada hasta entonces. — ProPatient",
			patient.FirstName, doctor.FullName, when,
		)
		vars := map[string]string{"1": patient.FirstName, "2": doctor.FullName, "3": when}
		if err := whatsapp.SendWithFallback(ctx, waClient, patient.Phone, waTemplates.BookingPatient, vars, body); err != nil {
			log.Printf("⚠️ No se pudo enviar el WhatsApp de confirmación al paciente (cita %d): %v", appointment.ID, err)
		} else {
			patientNotified = true
		}
	}

	if doctor.Phone != "" {
		reason := appointment.Reason
		if reason == "" {
			reason = "Sin motivo especificado"
		}
		reasonPart := ""
		if appointment.Reason != "" {
			reasonPart = fmt.Sprintf(" Motivo: %s.", appointment.Reason)
		}
		body := fmt.Sprintf(
			"Nueva solicitud de cita: %s %s, %s.%s Revísala en tu panel de ProPatient, en \"Solicitudes de Cita Nuevas\".",
			patient.FirstName, patient.LastName, when, reasonPart,
		)
		vars := map[string]string{
			"1": fmt.Sprintf("%s %s", patient.FirstName, patient.LastName),
			"2": when,
			"3": reason,
		}
		if err := whatsapp.SendWithFallback(ctx, waClient, doctor.Phone, waTemplates.BookingDoctor, vars, body); err != nil {
			log.Printf("⚠️ No se pudo enviar el WhatsApp de nueva solicitud al doctor %d: %v", doctor.ID, err)
		}
	}

	return patientNotified
}

// pushNotificationPayload es el JSON que recibe el service worker del
// frontend en el evento "push" (ver propatient-frontend/src/sw.ts) — title/
// body arman la notificación nativa, url es a dónde navega la app al
// tocarla.
type pushNotificationPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

// sendPublicBookingPush avisa al doctor, por notificación push (PWA), de
// una nueva solicitud de cita — el equivalente por push de
// sendPublicBookingWhatsApp. Mejor esfuerzo, mismo criterio que el resto
// de los avisos: nunca bloquea ni revierte la reserva si no hay
// suscripciones o el envío falla.
func sendPublicBookingPush(ctx context.Context, db *gorm.DB, pushClient webpush.Client, doctor models.Doctor, patient models.Patient, appointment models.Appointment) {
	if pushClient == nil {
		return
	}
	when := auth.FormatSpanishDateTimeShort(appointment.AppointmentDateTime)
	payload, err := json.Marshal(pushNotificationPayload{
		Title: "Nueva solicitud de cita",
		Body:  fmt.Sprintf("%s %s — %s", patient.FirstName, patient.LastName, when),
		// "Solicitudes de Cita Nuevas" vive en el dashboard de inicio
		// (AppointmentTracking.tsx, ruta "inicio" en App.tsx) — mismo
		// destino que el mensaje de WhatsApp al doctor le indica ir a ver.
		URL: "/inicio",
	})
	if err != nil {
		log.Printf("⚠️ No se pudo armar el payload de la notificación push (cita %d): %v", appointment.ID, err)
		return
	}
	webpush.SendToDoctor(ctx, db, pushClient, doctor.ID, payload)
}

// findOrCreatePublicBookingPatient decide a qué paciente pertenece una
// solicitud pública, con dos niveles de coincidencia para evitar
// duplicados:
//
//  1. Primero busca entre los pacientes YA vinculados a este doctor, por
//     correo o por teléfono — es el caso más común de duplicidad: el
//     paciente ya está en la agenda de este consultorio pero escribió el
//     correo con mayúsculas distintas, o directamente puso otro correo
//     pero el mismo teléfono que ya tenía registrado.
//  2. Si no aparece ahí, busca globalmente por correo (puede existir por
//     otro doctor, o por otra solicitud pública anterior) y lo reutiliza.
//  3. Si tampoco existe, crea un paciente nuevo.
//
// El segundo booleano indica si el paciente ya estaba vinculado a este
// doctor (para no volver a intentar vincularlo).
func findOrCreatePublicBookingPatient(db *gorm.DB, doctor models.Doctor, req publicAppointmentRequest) (*models.Patient, bool, error) {
	email := strings.ToLower(strings.TrimSpace(req.PatientEmail))
	phone := strings.TrimSpace(req.PatientPhone)

	var patient models.Patient
	err := db.Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
		Where("doctor_patients.doctor_id = ? AND (LOWER(patients.email) = ? OR patients.phone = ?)", doctor.ID, email, phone).
		First(&patient).Error
	if err == nil {
		return &patient, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	err = db.Where("LOWER(email) = ?", email).First(&patient).Error
	if err == nil {
		return &patient, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	patient = models.Patient{
		FirstName: req.PatientFirstName,
		LastName:  req.PatientLastName,
		Phone:     phone,
		Email:     email,
		IsMinor:   req.IsMinorPatient,
	}
	if err := db.Create(&patient).Error; err != nil {
		return nil, false, err
	}
	return &patient, false, nil
}

var slugNonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// generateDoctorSlug arma el identificador de la URL pública de un doctor
// (/dr/:slug) a partir de su nombre. Siempre incluye el ID al final: es la
// forma más simple de garantizar que nunca choque con el de otro doctor,
// sin tener que hacer una consulta extra a la base de datos para revisar
// colisiones.
func generateDoctorSlug(fullName string, doctorID uint) string {
	normalized := strings.ToLower(strings.TrimSpace(fullName))
	normalized = removeAccents(normalized)
	normalized = slugNonAlphanumeric.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		normalized = "doctor"
	}
	return normalized + "-" + strconv.FormatUint(uint64(doctorID), 10)
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ü", "u",
)

func removeAccents(s string) string {
	return accentReplacer.Replace(s)
}
