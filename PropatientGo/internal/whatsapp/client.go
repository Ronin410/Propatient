// Package whatsapp manda mensajes de WhatsApp vía la API de Twilio. Sigue
// el mismo patrón de cliente inyectable que internal/billing,
// internal/googlecalendar, internal/storage y internal/geocoding: sin
// credenciales configuradas el cliente real nunca se construye y el envío
// simplemente se salta (mejor esfuerzo, nunca bloquea el flujo principal).
package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	AccountSID string
	AuthToken  string
	// FromNumber ya con el prefijo "whatsapp:", ej. "whatsapp:+14155238886"
	// (el número de sandbox de Twilio, o el número de negocio aprobado).
	FromNumber string
}

func LoadConfigFromEnv() Config {
	return Config{
		AccountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		AuthToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
		FromNumber: os.Getenv("TWILIO_WHATSAPP_FROM"),
	}
}

func (c Config) IsConfigured() bool {
	return c.AccountSID != "" && c.AuthToken != "" && c.FromNumber != ""
}

// Templates guarda el ContentSid (Twilio Content API) de cada plantilla de
// WhatsApp aprobada por Meta, una por tipo de aviso. Fuera de la ventana de
// 24h en que el paciente/doctor te escribió primero, Meta EXIGE que los
// mensajes que el negocio inicia (recordatorios, confirmaciones, avisos)
// vayan por una plantilla pre-aprobada — no por texto libre. Mientras una
// plantilla todavía no está dada de alta o aprobada, su ContentSid queda
// vacío y SendWithFallback cae automáticamente al texto libre de siempre
// (funciona en sandbox y dentro de una sesión abierta, igual que antes de
// este cambio) — así se puede desplegar el código ya y activar cada
// plantilla en cuanto Meta la apruebe, sin tener que coordinar un deploy.
//
// Además de ser lo único que funciona fuera del sandbox, las plantillas
// categoría "utility" (transaccionales, que es lo que son todos estos
// avisos) cuestan notablemente menos que "marketing" en el cobro de Meta —
// por eso vale la pena darlas de alta aunque el texto libre "funcione" hoy.
type Templates struct {
	BookingPatient       string // TWILIO_TEMPLATE_BOOKING_PATIENT
	BookingDoctor        string // TWILIO_TEMPLATE_BOOKING_DOCTOR
	AppointmentConfirmed string // TWILIO_TEMPLATE_APPOINTMENT_CONFIRMED
	AppointmentRejected  string // TWILIO_TEMPLATE_APPOINTMENT_REJECTED
	ReminderPatient      string // TWILIO_TEMPLATE_REMINDER_PATIENT
	ReviewRequest        string // TWILIO_TEMPLATE_REVIEW_REQUEST
	FollowUp             string // TWILIO_TEMPLATE_FOLLOWUP
}

func LoadTemplatesFromEnv() Templates {
	return Templates{
		BookingPatient:       os.Getenv("TWILIO_TEMPLATE_BOOKING_PATIENT"),
		BookingDoctor:        os.Getenv("TWILIO_TEMPLATE_BOOKING_DOCTOR"),
		AppointmentConfirmed: os.Getenv("TWILIO_TEMPLATE_APPOINTMENT_CONFIRMED"),
		AppointmentRejected:  os.Getenv("TWILIO_TEMPLATE_APPOINTMENT_REJECTED"),
		ReminderPatient:      os.Getenv("TWILIO_TEMPLATE_REMINDER_PATIENT"),
		ReviewRequest:        os.Getenv("TWILIO_TEMPLATE_REVIEW_REQUEST"),
		FollowUp:             os.Getenv("TWILIO_TEMPLATE_FOLLOWUP"),
	}
}

// Client abstrae el envío de un mensaje de WhatsApp, para poder mockearlo
// en tests de integración sin llamar a la API real de Twilio.
type Client interface {
	SendMessage(ctx context.Context, toPhone, body string) error
	// SendTemplate manda un mensaje usando una plantilla aprobada (Twilio
	// Content API). variables son los placeholders numerados de la
	// plantilla ("1", "2", ... en el orden en que Meta los aprobó).
	SendTemplate(ctx context.Context, toPhone, contentSID string, variables map[string]string) error
}

// SendWithFallback manda por la plantilla aprobada si contentSID no está
// vacío; si todavía no hay plantilla configurada para este aviso, cae al
// texto libre (fallbackBody) — mismo comportamiento que tenía la app antes
// de que existieran las plantillas. client puede ser nil (Twilio no
// configurado), en cuyo caso no hace nada, igual que los call sites hacían
// antes de este cambio.
func SendWithFallback(ctx context.Context, client Client, toPhone, contentSID string, variables map[string]string, fallbackBody string) error {
	if client == nil {
		return nil
	}
	if contentSID != "" {
		return client.SendTemplate(ctx, toPhone, contentSID, variables)
	}
	return client.SendMessage(ctx, toPhone, fallbackBody)
}

type twilioClient struct {
	httpClient *http.Client
	accountSID string
	authToken  string
	fromNumber string
}

// NewClient construye el cliente real de Twilio. Solo se debe llamar si
// cfg.IsConfigured() es true.
func NewClient(cfg Config) Client {
	return &twilioClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		accountSID: cfg.AccountSID,
		authToken:  cfg.AuthToken,
		fromNumber: cfg.FromNumber,
	}
}

func (t *twilioClient) SendMessage(ctx context.Context, toPhone, body string) error {
	to := NormalizeE164(toPhone)
	if to == "" {
		return errors.New("número de teléfono vacío o inválido")
	}

	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.accountSID)
	form := url.Values{}
	form.Set("From", t.fromNumber)
	form.Set("To", "whatsapp:"+to)
	form.Set("Body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(t.accountSID, t.authToken)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errBody struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		if errBody.Message != "" {
			return fmt.Errorf("twilio respondió %d: %s", resp.StatusCode, errBody.Message)
		}
		return fmt.Errorf("twilio respondió con error %d", resp.StatusCode)
	}
	return nil
}

func (t *twilioClient) SendTemplate(ctx context.Context, toPhone, contentSID string, variables map[string]string) error {
	to := NormalizeE164(toPhone)
	if to == "" {
		return errors.New("número de teléfono vacío o inválido")
	}

	varsJSON, err := json.Marshal(variables)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.accountSID)
	form := url.Values{}
	form.Set("From", t.fromNumber)
	form.Set("To", "whatsapp:"+to)
	form.Set("ContentSid", contentSID)
	form.Set("ContentVariables", string(varsJSON))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(t.accountSID, t.authToken)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errBody struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		if errBody.Message != "" {
			return fmt.Errorf("twilio respondió %d: %s", resp.StatusCode, errBody.Message)
		}
		return fmt.Errorf("twilio respondió con error %d", resp.StatusCode)
	}
	return nil
}

var nonDigits = regexp.MustCompile(`[^0-9]`)

// NormalizeE164 arma un teléfono en formato internacional E.164 (+52...)
// para WhatsApp. Los teléfonos de doctores/pacientes se guardan como texto
// libre sin código de país (ej. "5512345678"); si ya viene con "+" se
// respeta tal cual (limpiando espacios/guiones), si no, se asume México
// (+52) — decisión explícita del producto, ver .env.example.
func NormalizeE164(phone string) string {
	trimmed := strings.TrimSpace(phone)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "+") {
		digits := nonDigits.ReplaceAllString(trimmed[1:], "")
		if digits == "" {
			return ""
		}
		return "+" + digits
	}
	digits := nonDigits.ReplaceAllString(trimmed, "")
	if digits == "" {
		return ""
	}
	return "+52" + digits
}
