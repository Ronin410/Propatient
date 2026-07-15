package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

const resendAPIURL = "https://api.resend.com/emails"

// SendEmail manda un correo HTML vía la API HTTP de Resend (RESEND_API_KEY).
// Extraído como función reutilizable para que otros flujos (invitación de
// personal, solicitud de cita pública, recordatorios) no dupliquen la
// llamada a la API.
//
// Se usa una API HTTP y no SMTP a propósito: varios hostings (Render
// incluido) bloquean las conexiones salientes por el puerto SMTP, lo que
// hacía que el envío por Gmail se quedara colgado hasta agotar el timeout
// de red en vez de fallar — ver sendViaResend.
//
// Sin RESEND_API_KEY, falla de inmediato en vez de intentar la llamada.
func SendEmail(toEmail, subject, htmlBody string) error {
	return sendViaResend(toEmail, subject, htmlBody)
}

// sendViaResend hace la llamada HTTP real a Resend. Compartida por
// SendEmail y SendValidationEmail para no duplicar la lógica de la
// petición.
//
// RESEND_FROM_EMAIL debe ser un remitente de un dominio verificado en
// Resend (ej. "ProPatient <notificaciones@tudominio.com>"); si no está
// configurada, cae al dominio de pruebas de Resend
// ("onboarding@resend.dev"), que SOLO entrega correos a la cuenta dueña de
// la API key — sirve para probar el flujo, no para pacientes reales.
func sendViaResend(toEmail, subject, htmlBody string) error {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return errors.New("RESEND_API_KEY no está configurada")
	}
	from := os.Getenv("RESEND_FROM_EMAIL")
	if from == "" {
		from = "ProPatient <onboarding@resend.dev>"
	}

	payload, err := json.Marshal(map[string]any{
		"from":    from,
		"to":      []string{toEmail},
		"subject": subject,
		"html":    htmlBody,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, resendAPIURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
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
			return fmt.Errorf("resend respondió %d: %s", resp.StatusCode, errBody.Message)
		}
		return fmt.Errorf("resend respondió con error %d", resp.StatusCode)
	}
	return nil
}

var spanishMonths = [...]string{
	"enero", "febrero", "marzo", "abril", "mayo", "junio",
	"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre",
}

// AppTimeZone es la zona horaria del consultorio (Culiacán/Mazatlán,
// UTC-7), la misma que el frontend ya usaba de forma fija para mostrar
// horarios (ver propatient-frontend/src/utils/dateFormatter.ts). Todavía
// no hay soporte para que cada doctor configure su propia zona horaria —
// si eso se agrega, este valor debería salir del perfil del doctor en vez
// de estar fijo aquí.
const AppTimeZone = "America/Mazatlan"

// FormatSpanishDateTime da una fecha/hora legible en español para el cuerpo
// de los correos/WhatsApp automáticos (confirmaciones, recordatorios), ej.
// "15 de julio de 2026, 10:00 a.m.".
//
// AppointmentDateTime se guarda siempre en UTC (mismo criterio que el
// resto del backend); aquí se convierte a AppTimeZone antes de formatear
// para que la hora que lee el paciente/doctor sea la hora real de su
// consultorio, no la hora UTC cruda (ese fue exactamente el bug: una cita
// de las 10:00 a.m. se guardaba como 17:00 UTC, y sin esta conversión el
// correo mostraba "5:00 p.m.").
func FormatSpanishDateTime(t time.Time) string {
	if loc, err := time.LoadLocation(AppTimeZone); err == nil {
		t = t.In(loc)
	}

	hour12, period := t.Hour(), "a.m."
	switch {
	case hour12 == 0:
		hour12 = 12
	case hour12 == 12:
		period = "p.m."
	case hour12 > 12:
		hour12 -= 12
		period = "p.m."
	}
	return fmt.Sprintf("%d de %s de %d, %02d:%02d %s", t.Day(), spanishMonths[t.Month()-1], t.Year(), hour12, t.Minute(), period)
}
