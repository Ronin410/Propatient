// Package googlecalendar aísla toda la integración con la API de Google
// Calendar detrás de una interfaz chica (Client), para poder:
//  1. Probar los handlers de citas sin llamar a Google de verdad (con un
//     mock que implemente la misma interfaz).
//  2. Que un fallo de Google Calendar nunca tumbe una operación real sobre
//     una cita — los handlers tratan la sincronización como "best effort".
package googlecalendar

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// EventInput son los datos mínimos necesarios para reflejar una cita como
// evento de calendario.
type EventInput struct {
	Summary     string
	Description string
	Start       time.Time
	End         time.Time
}

// Client abstrae las operaciones de Google Calendar que usa la app.
type Client interface {
	CreateEvent(ctx context.Context, refreshToken string, ev EventInput) (eventID string, err error)
	UpdateEvent(ctx context.Context, refreshToken string, eventID string, ev EventInput) error
	DeleteEvent(ctx context.Context, refreshToken string, eventID string) error
}

// Config lee la configuración OAuth de variables de entorno. Si falta el
// Client Secret, IsConfigured() devuelve false y el resto de la
// funcionalidad queda deshabilitada con gracia (no crashea el servidor).
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func LoadConfigFromEnv() Config {
	// GOOGLE_CALENDAR_CLIENT_ID es un OAuth Client ID propio, separado del
	// GOOGLE_CLIENT_ID que usa el login (ese es de tipo "público", sin
	// secret, y no tiene el scope de Calendar). Si algún día se decide
	// reutilizar el mismo client para ambas cosas, basta con apuntar esta
	// variable al mismo valor que GOOGLE_CLIENT_ID.
	return Config{
		ClientID:     os.Getenv("GOOGLE_CALENDAR_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_CALENDAR_REDIRECT_URI"),
	}
}

func (c Config) IsConfigured() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

const calendarScope = "https://www.googleapis.com/auth/calendar.events"

func (c Config) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Scopes:       []string{calendarScope},
		Endpoint:     googleoauth.Endpoint,
	}
}

// AuthURL arma la URL de consentimiento de Google. "state" debe ser un valor
// firmado/verificable (en este proyecto, un JWT propio) que el callback usa
// para saber a qué doctor pertenece el código de autorización.
func (c Config) AuthURL(state string) string {
	return c.oauthConfig().AuthCodeURL(
		state,
		oauth2.AccessTypeOffline, // pide refresh_token, no solo access_token
		oauth2.ApprovalForce,     // fuerza la pantalla de consentimiento para garantizar el refresh_token
	)
}

// Exchange intercambia el "code" que Google mandó al callback por un
// refresh_token de larga duración.
func (c Config) Exchange(ctx context.Context, code string) (refreshToken string, err error) {
	token, err := c.oauthConfig().Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("error al intercambiar el código de Google: %w", err)
	}
	if token.RefreshToken == "" {
		return "", fmt.Errorf("Google no devolvió un refresh_token (revisa que access_type=offline y prompt=consent estén presentes)")
	}
	return token.RefreshToken, nil
}

// realClient implementa Client usando la API real de Google Calendar.
type realClient struct {
	cfg Config
}

func NewClient(cfg Config) Client {
	return &realClient{cfg: cfg}
}

func (r *realClient) service(ctx context.Context, refreshToken string) (*calendar.Service, error) {
	tokenSource := r.cfg.oauthConfig().TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	return calendar.NewService(ctx, option.WithTokenSource(tokenSource))
}

func toGoogleEvent(ev EventInput) *calendar.Event {
	return &calendar.Event{
		Summary:     ev.Summary,
		Description: ev.Description,
		Start:       &calendar.EventDateTime{DateTime: ev.Start.Format(time.RFC3339)},
		End:         &calendar.EventDateTime{DateTime: ev.End.Format(time.RFC3339)},
	}
}

func (r *realClient) CreateEvent(ctx context.Context, refreshToken string, ev EventInput) (string, error) {
	svc, err := r.service(ctx, refreshToken)
	if err != nil {
		return "", err
	}
	created, err := svc.Events.Insert("primary", toGoogleEvent(ev)).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	return created.Id, nil
}

func (r *realClient) UpdateEvent(ctx context.Context, refreshToken string, eventID string, ev EventInput) error {
	svc, err := r.service(ctx, refreshToken)
	if err != nil {
		return err
	}
	_, err = svc.Events.Update("primary", eventID, toGoogleEvent(ev)).Context(ctx).Do()
	return err
}

func (r *realClient) DeleteEvent(ctx context.Context, refreshToken string, eventID string) error {
	svc, err := r.service(ctx, refreshToken)
	if err != nil {
		return err
	}
	err = svc.Events.Delete("primary", eventID).Context(ctx).Do()
	return err
}
