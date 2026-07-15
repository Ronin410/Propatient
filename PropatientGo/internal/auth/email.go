package auth

import (
	"errors"
	"net/smtp"
	"os"
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
