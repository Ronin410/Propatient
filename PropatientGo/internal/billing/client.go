// Package billing integra el cobro de suscripción a los doctores vía
// Stripe Checkout (alta de la suscripción) y el Customer Portal (que el
// propio doctor cancele/actualice su método de pago sin que tengamos que
// construir esa UI). Sigue el mismo patrón de cliente inyectable que
// internal/googlecalendar e internal/storage: sin credenciales
// configuradas, el cliente real nunca se construye y las rutas de
// facturación devuelven 503 en vez de fallar de forma confusa.
package billing

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v81/checkout/session"
)

// TrialDuration es la prueba gratis de todo doctor nuevo (14 días, decisión
// del producto). Usado al dar de alta la cuenta (ver auth.RegisterDoctor y
// auth.GoogleLoginHandler) y por RequireActiveSubscription para decidir si
// aún está dentro de la prueba.
const TrialDuration = 14 * 24 * time.Hour

type Config struct {
	SecretKey     string
	PriceID       string
	WebhookSecret string
}

func LoadConfigFromEnv() Config {
	return Config{
		SecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		PriceID:       os.Getenv("STRIPE_PRICE_ID"),
		WebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
}

func (c Config) IsConfigured() bool {
	return c.SecretKey != "" && c.PriceID != ""
}

// CheckoutParams son los datos necesarios para armar una sesión de Stripe
// Checkout para un doctor específico.
type CheckoutParams struct {
	DoctorID           uint
	CustomerEmail      string
	ExistingCustomerID string // si ya existe, reutiliza el mismo Customer de Stripe
	SuccessURL         string
	CancelURL          string
}

// Client abstrae las dos llamadas a la API de Stripe que necesita la app:
// crear una sesión de Checkout (para suscribirse) y una sesión del
// Customer Portal (para gestionar/cancelar). Permite mockear en tests de
// integración sin llamar a la red real.
type Client interface {
	CreateCheckoutSession(ctx context.Context, params CheckoutParams) (checkoutURL string, err error)
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (portalURL string, err error)
}

type realClient struct {
	priceID string
}

// NewClient construye el cliente real de Stripe. Solo se debe llamar si
// cfg.IsConfigured() es true.
func NewClient(cfg Config) Client {
	stripe.Key = cfg.SecretKey
	return &realClient{priceID: cfg.PriceID}
}

func (r *realClient) CreateCheckoutSession(ctx context.Context, params CheckoutParams) (string, error) {
	sParams := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:        stripe.String(params.SuccessURL),
		CancelURL:         stripe.String(params.CancelURL),
		ClientReferenceID: stripe.String(strconv.FormatUint(uint64(params.DoctorID), 10)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(r.priceID), Quantity: stripe.Int64(1)},
		},
	}
	if params.ExistingCustomerID != "" {
		sParams.Customer = stripe.String(params.ExistingCustomerID)
	} else {
		sParams.CustomerEmail = stripe.String(params.CustomerEmail)
	}
	sParams.Context = ctx

	sess, err := checkoutsession.New(sParams)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

func (r *realClient) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	pParams := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}
	pParams.Context = ctx

	sess, err := session.New(pParams)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}
