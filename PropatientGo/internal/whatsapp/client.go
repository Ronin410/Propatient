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

// Client abstrae el envío de un mensaje de WhatsApp, para poder mockearlo
// en tests de integración sin llamar a la API real de Twilio.
type Client interface {
	SendMessage(ctx context.Context, toPhone, body string) error
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
