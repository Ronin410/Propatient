// Package webpush manda notificaciones push (Web Push/VAPID) al navegador
// instalado de un doctor. Sigue el mismo patrón de cliente inyectable que
// internal/whatsapp, internal/storage y internal/billing: sin credenciales
// configuradas el cliente real nunca se construye y el envío simplemente se
// salta (mejor esfuerzo, nunca bloquea el flujo principal).
package webpush

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	webpushgo "github.com/SherClockHolmes/webpush-go"
)

type Config struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	// VAPIDSubject identifica al remitente ante el navegador/proveedor push
	// (Chrome, Firefox, etc.) — un "mailto:" o una URL, exigido por el
	// estándar Web Push.
	VAPIDSubject string
}

func LoadConfigFromEnv() Config {
	return Config{
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:    os.Getenv("VAPID_SUBJECT"),
	}
}

func (c Config) IsConfigured() bool {
	return c.VAPIDPublicKey != "" && c.VAPIDPrivateKey != "" && c.VAPIDSubject != ""
}

// Subscription es la suscripción push de un navegador/dispositivo concreto
// — el mismo objeto que devuelve PushManager.subscribe() en el frontend,
// sin depender de internal/models para que este paquete se pueda probar
// aislado (mismo criterio que internal/whatsapp, que tampoco importa
// models).
type Subscription struct {
	Endpoint  string
	P256dhKey string
	AuthKey   string
}

// ErrSubscriptionExpired distingue el caso en que el navegador ya invalidó
// la suscripción (el proveedor push responde 404/410) de un error de red o
// de configuración — quien llama usa esto para saber cuándo debe borrar la
// fila en vez de solo reintentar después (ver SendToDoctor).
var ErrSubscriptionExpired = errors.New("la suscripción push ya no es válida (404/410)")

// Client abstrae el envío de una notificación push, para poder mockearlo
// en tests de integración sin llamar a un proveedor push real.
type Client interface {
	SendNotification(ctx context.Context, sub Subscription, payload []byte) error
}

type vapidClient struct {
	publicKey  string
	privateKey string
	subject    string
}

// NewClient construye el cliente real. Solo se debe llamar si
// cfg.IsConfigured() es true.
func NewClient(cfg Config) Client {
	return &vapidClient{
		publicKey:  cfg.VAPIDPublicKey,
		privateKey: cfg.VAPIDPrivateKey,
		subject:    cfg.VAPIDSubject,
	}
}

func (c *vapidClient) SendNotification(ctx context.Context, sub Subscription, payload []byte) error {
	resp, err := webpushgo.SendNotificationWithContext(ctx, payload, &webpushgo.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpushgo.Keys{
			P256dh: sub.P256dhKey,
			Auth:   sub.AuthKey,
		},
	}, &webpushgo.Options{
		Subscriber:      c.subject,
		VAPIDPublicKey:  c.publicKey,
		VAPIDPrivateKey: c.privateKey,
		TTL:             60 * 60, // 1h: si el navegador no está en línea, no vale la pena reintentar por más tiempo un aviso de "nueva solicitud de cita".
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return ErrSubscriptionExpired
	}
	if resp.StatusCode >= 300 {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return fmt.Errorf("el proveedor push respondió %d: %s", resp.StatusCode, buf.String())
	}
	return nil
}
