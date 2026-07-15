package auth

import (
	"errors"
	"fmt"
	"net/smtp"
	"os"
	"time"
)

// SendEmail manda un correo HTML simple usando las mismas credenciales SMTP
// que SendValidationEmail (SMTP_EMAIL / SMTP_PASSWORD). Extraído como
// función reutilizable para que otros flujos (ej. invitación de personal)
// no dupliquen la construcción del mensaje.
//
// Sin esas dos variables, falla de inmediato en vez de intentar conectarse
// a smtp.gmail.com: sin credenciales esa conexión igual va a fallar, pero
// tarda hasta el timeout de red completo (decenas de segundos) en darse
// cuenta, y bloquea la petición HTTP que llamó a esta función mientras tanto.
func SendEmail(toEmail, subject, htmlBody string) error {
	senderEmail := os.Getenv("SMTP_EMAIL")
	senderPass := os.Getenv("SMTP_PASSWORD")
	if senderEmail == "" || senderPass == "" {
		return errors.New("SMTP_EMAIL/SMTP_PASSWORD no están configuradas")
	}

	smtpAuth := smtp.PlainAuth("", senderEmail, senderPass, SMTPServer)

	msg := []byte("Subject: " + subject + "\n" +
		"MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n" +
		htmlBody)

	addr := SMTPServer + ":" + SMTPPort
	return smtp.SendMail(addr, smtpAuth, senderEmail, []string{toEmail}, msg)
}

var spanishMonths = [...]string{
	"enero", "febrero", "marzo", "abril", "mayo", "junio",
	"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre",
}

// FormatSpanishDateTime da una fecha/hora legible en español para el cuerpo
// de los correos automáticos (confirmaciones, recordatorios), ej. "15 de
// julio de 2026, 10:00 a.m.". Siempre en la hora tal cual está guardada
// (UTC, mismo criterio que el resto del backend para AppointmentDateTime).
func FormatSpanishDateTime(t time.Time) string {
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
