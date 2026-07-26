package handlers

import (
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"propatient-api/internal/models"
	"propatient-api/internal/whatsapp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// twilioAuthTokenForWebhook y twilioWebhookURL leen directo del entorno
// (mismo criterio que frontendRedirectBase en google_calendar_handler.go)
// en vez de agregarle un parámetro nuevo a NewRouterWithDeps — evita
// tocar la firma de esa función y, con ella, todos los tests que ya la
// llaman con la lista actual de argumentos.
func twilioAuthTokenForWebhook() string {
	return os.Getenv("TWILIO_AUTH_TOKEN")
}

// twilioWebhookURL debe ser EXACTAMENTE la URL pública dada de alta en la
// consola de Twilio para este webhook (Sender > Configuration > "When a
// message comes in") — ver el comentario largo en
// whatsapp.ValidateInboundSignature sobre por qué no se reconstruye a
// partir de la petición.
func twilioWebhookURL() string {
	return strings.TrimSpace(os.Getenv("TWILIO_WEBHOOK_URL"))
}

var inboundPhoneDigits = regexp.MustCompile(`[^0-9]`)

// candidatePhoneFormats arma las variantes con las que un mismo teléfono
// mexicano puede haber quedado guardado en Patient.Phone/Doctor.Phone —
// esos campos son texto libre capturado a mano (ver el comentario de
// whatsapp.NormalizeE164), casi siempre sin código de país, mientras que
// lo que manda Twilio en "From" siempre viene en E.164 completo y, para
// números mexicanos, WhatsApp además intercala un "1" extra después del
// 52 (herencia de cuando los celulares en México llevaban ese prefijo).
// Sin generar estas variantes, un doctor/paciente que guardó su teléfono
// como "5512345678" nunca haría match contra el "+5215512345678" que
// realmente llega en el webhook.
func candidatePhoneFormats(fromPhone string) []string {
	trimmed := strings.TrimPrefix(fromPhone, "whatsapp:")
	digits := inboundPhoneDigits.ReplaceAllString(trimmed, "")

	set := map[string]bool{}
	add := func(v string) {
		if v != "" {
			set[v] = true
		}
	}
	add(trimmed)
	add(digits)

	switch {
	case strings.HasPrefix(digits, "521") && len(digits) == 13:
		local := digits[3:]
		add(local)
		add("52" + local)
		add("+52" + local)
	case strings.HasPrefix(digits, "52") && len(digits) == 12:
		local := digits[2:]
		add(local)
		add("+" + digits)
	case len(digits) == 10:
		add("52" + digits)
		add("+52" + digits)
	}

	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	return out
}

// classifyInboundWhatsApp decide si un teléfono que le escribió a
// ProPatient es un doctor pidiendo soporte, un paciente conocido, o un
// número que no reconocemos — y de paso arma el contexto (nombre/id) que
// el superadmin va a ver junto al mensaje. Nunca decide "de qué doctor"
// es un paciente para restringir quién puede LEER el mensaje (ambas
// categorías son de solo-superadmin), solo para mostrar contexto: se usa
// la cita más reciente del paciente como mejor esfuerzo.
func classifyInboundWhatsApp(db *gorm.DB, fromPhone string) (category string, doctorID *uint, patientID *uint) {
	candidates := candidatePhoneFormats(fromPhone)

	var doctor models.Doctor
	if err := db.Where("phone IN ? AND phone <> ''", candidates).First(&doctor).Error; err == nil {
		id := doctor.ID
		return "DOCTOR_SUPPORT", &id, nil
	}

	var patient models.Patient
	if err := db.Where("phone IN ? AND phone <> ''", candidates).First(&patient).Error; err == nil {
		pid := patient.ID
		var appt models.Appointment
		if err := db.Where("patient_id = ?", patient.ID).Order("created_at DESC").First(&appt).Error; err == nil {
			did := appt.DoctorID
			return "PATIENT", &did, &pid
		}
		return "PATIENT", nil, &pid
	}

	return "PATIENT", nil, nil
}

// TwilioInboundWebhook recibe las respuestas de WhatsApp al número de
// ProPatient — tanto de pacientes contestando un aviso como de doctores
// pidiendo soporte. Es pública (Twilio la llama directo, sin sesión de
// usuario): la firma X-Twilio-Signature es lo único que la autentica, así
// que va fuera de /auth y de cualquier grupo protegido (mismo criterio
// que StripeWebhook con Stripe-Signature).
func TwilioInboundWebhook(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authToken := twilioAuthTokenForWebhook()
		webhookURL := twilioWebhookURL()
		if authToken == "" || webhookURL == "" {
			// Sin configurar todavía: no hay forma segura de validar la
			// firma, así que se rechaza en vez de aceptar mensajes sin
			// verificar. No es un error del que llama (Twilio), es
			// configuración pendiente del lado de ProPatient.
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Webhook de WhatsApp no configurado"})
			return
		}

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No se pudo leer la petición"})
			return
		}
		c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		if err := c.Request.ParseForm(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Formulario inválido"})
			return
		}

		params := make(map[string]string, len(c.Request.PostForm))
		for k, v := range c.Request.PostForm {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}

		signature := c.GetHeader("X-Twilio-Signature")
		if !whatsapp.ValidateInboundSignature(authToken, webhookURL, params, signature) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Firma inválida"})
			return
		}

		from := params["From"]
		body := params["Body"]
		messageSid := params["MessageSid"]
		if from == "" || messageSid == "" {
			c.Data(http.StatusOK, "application/xml", []byte(`<?xml version="1.0" encoding="UTF-8"?><Response></Response>`))
			return
		}

		// Idempotencia: Twilio puede reintentar el mismo webhook si la
		// primera respuesta tarda o se pierde en el camino.
		var existing models.WhatsAppMessage
		if err := db.Where("twilio_sid = ?", messageSid).First(&existing).Error; err == nil {
			c.Data(http.StatusOK, "application/xml", []byte(`<?xml version="1.0" encoding="UTF-8"?><Response></Response>`))
			return
		}

		category, doctorID, patientID := classifyInboundWhatsApp(db, from)
		phone := whatsapp.NormalizeE164(strings.TrimPrefix(from, "whatsapp:"))

		msg := models.WhatsAppMessage{
			Direction: "INBOUND",
			Category:  category,
			Phone:     phone,
			Body:      body,
			TwilioSid: &messageSid,
			DoctorID:  doctorID,
			PatientID: patientID,
		}
		if err := db.Create(&msg).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar el mensaje"})
			return
		}

		c.Data(http.StatusOK, "application/xml", []byte(`<?xml version="1.0" encoding="UTF-8"?><Response></Response>`))
	}
}

// threadSummary es lo que ve el superadmin en la lista de conversaciones,
// sin tener que abrir cada una para saber de qué se trata.
type threadSummary struct {
	Phone           string    `json:"phone"`
	Category        string    `json:"category"`
	LastMessageBody string    `json:"lastMessageBody"`
	LastMessageAt   time.Time `json:"lastMessageAt"`
	LastDirection   string    `json:"lastDirection"`
	UnreadCount     int64     `json:"unreadCount"`
	DoctorID        *uint     `json:"doctorId"`
	DoctorName      string    `json:"doctorName,omitempty"`
	PatientID       *uint     `json:"patientId"`
	PatientName     string    `json:"patientName,omitempty"`
}

// ListWhatsAppThreads agrupa los mensajes por teléfono dentro de la
// categoría pedida (PATIENT o DOCTOR_SUPPORT) — cada teléfono distinto es
// una "conversación". Sin esto el superadmin vería una lista plana de
// mensajes sueltos en vez de hilos, sin poder saber de un vistazo cuáles
// ya se contestaron.
func ListWhatsAppThreads(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		category := c.DefaultQuery("category", "PATIENT")
		if category != "PATIENT" && category != "DOCTOR_SUPPORT" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category debe ser PATIENT o DOCTOR_SUPPORT"})
			return
		}

		var messages []models.WhatsAppMessage
		if err := db.Where("category = ?", category).Order("created_at DESC").Find(&messages).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron cargar los mensajes"})
			return
		}

		order := make([]string, 0)
		byPhone := make(map[string]*threadSummary)
		unread := make(map[string]int64)

		for _, m := range messages {
			if _, ok := byPhone[m.Phone]; !ok {
				byPhone[m.Phone] = &threadSummary{
					Phone:           m.Phone,
					Category:        m.Category,
					LastMessageBody: m.Body,
					LastMessageAt:   m.CreatedAt,
					LastDirection:   m.Direction,
					DoctorID:        m.DoctorID,
					PatientID:       m.PatientID,
				}
				order = append(order, m.Phone)
			}
			if m.Direction == "INBOUND" && m.ReadAt == nil {
				unread[m.Phone]++
			}
		}

		threads := make([]threadSummary, 0, len(order))
		for _, phone := range order {
			t := byPhone[phone]
			t.UnreadCount = unread[phone]
			if t.DoctorID != nil {
				var doctor models.Doctor
				if db.Select("full_name").First(&doctor, *t.DoctorID).Error == nil {
					t.DoctorName = doctor.FullName
				}
			}
			if t.PatientID != nil {
				var patient models.Patient
				if db.Select("first_name, last_name").First(&patient, *t.PatientID).Error == nil {
					t.PatientName = strings.TrimSpace(patient.FirstName + " " + patient.LastName)
				}
			}
			threads = append(threads, *t)
		}

		c.JSON(http.StatusOK, threads)
	}
}

// GetWhatsAppThreadMessages regresa toda la conversación con un teléfono
// (orden cronológico) y marca como leídos los mensajes entrantes
// pendientes — abrir la conversación ES la acción de "leer".
func GetWhatsAppThreadMessages(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		phone := c.Param("phone")

		var messages []models.WhatsAppMessage
		if err := db.Where("phone = ?", phone).Order("created_at ASC").Find(&messages).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cargar la conversación"})
			return
		}

		now := time.Now().UTC()
		db.Model(&models.WhatsAppMessage{}).
			Where("phone = ? AND direction = ? AND read_at IS NULL", phone, "INBOUND").
			Update("read_at", now)

		c.JSON(http.StatusOK, messages)
	}
}

type replyWhatsAppInput struct {
	Message string `json:"message" binding:"required"`
}

// ReplyToWhatsAppThread manda una respuesta de texto libre al teléfono del
// hilo — dentro de la ventana de 24h que abrió el mensaje entrante, Meta
// no exige plantilla aprobada para esto (ver el comentario de
// whatsapp.SendWithFallback). Hereda la categoría/doctor/paciente del
// último mensaje ENTRANTE de ese teléfono, para que la conversación se
// vea completa y consistente en el hilo.
func ReplyToWhatsAppThread(db *gorm.DB, waClient whatsapp.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		phone := c.Param("phone")

		var input replyWhatsAppInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var lastInbound models.WhatsAppMessage
		if err := db.Where("phone = ? AND direction = ?", phone, "INBOUND").
			Order("created_at DESC").First(&lastInbound).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No hay ninguna conversación con ese teléfono"})
			return
		}

		if waClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WhatsApp no está configurado"})
			return
		}

		if err := whatsapp.SendWithFallback(c.Request.Context(), waClient, phone, "", nil, input.Message); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "No se pudo mandar el mensaje: " + err.Error()})
			return
		}

		reply := models.WhatsAppMessage{
			Direction: "OUTBOUND",
			Category:  lastInbound.Category,
			Phone:     phone,
			Body:      input.Message,
			DoctorID:  lastInbound.DoctorID,
			PatientID: lastInbound.PatientID,
		}
		if err := db.Create(&reply).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "El mensaje se mandó pero no se pudo guardar en el historial"})
			return
		}

		c.JSON(http.StatusCreated, reply)
	}
}
