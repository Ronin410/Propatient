// Package recaptcha verifica tokens de Google reCAPTCHA v3 en endpoints
// públicos sensibles a spam (ej. agendar cita sin cuenta), como una capa
// extra sobre el rate limiting por IP que ya existe.
package recaptcha

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"
)

const verifyURL = "https://www.google.com/recaptcha/api/siteverify"

// minScore: reCAPTCHA v3 no bloquea con un captcha visible, da un puntaje
// de 0.0 (probable bot) a 1.0 (probable humano). 0.5 es el umbral que la
// propia documentación de Google recomienda como punto de partida.
const minScore = 0.5

// Verify llama a la API de verificación de Google con el token que mandó el
// frontend. Sin RECAPTCHA_SECRET_KEY configurada, no hay nada que
// verificar — se considera aprobado (la petición sigue protegida por el
// rate limiting existente), igual que el resto de integraciones opcionales
// de esta app (WhatsApp, S3, Google Calendar): nunca bloquea el flujo
// completo solo porque una integración externa no esté configurada.
func Verify(ctx context.Context, token, action string) error {
	secret := os.Getenv("RECAPTCHA_SECRET_KEY")
	if secret == "" {
		return nil
	}
	if token == "" {
		return errors.New("no se pudo verificar que la solicitud viene de una persona real, recarga la página e intenta de nuevo")
	}

	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, nil)
	if err != nil {
		return err
	}
	req.URL.RawQuery = form.Encode()

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Un fallo de red hacia Google no debe tumbar el agendado de citas
		// reales — se deja pasar, protegido igual por el rate limiting.
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Success bool     `json:"success"`
		Score   float64  `json:"score"`
		Action  string   `json:"action"`
		Errors  []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	if !result.Success || result.Score < minScore || (action != "" && result.Action != action) {
		return errors.New("no pudimos verificar que la solicitud viene de una persona real, intenta de nuevo")
	}
	return nil
}
