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
	"strings"
	"time"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/subscription"
	"github.com/stripe/stripe-go/v81/subscriptionitem"
)

// TrialDuration es la prueba gratis de todo doctor nuevo (14 días, decisión
// del producto). Usado al dar de alta la cuenta (ver auth.RegisterDoctor y
// auth.GoogleLoginHandler) y por RequireActiveSubscription para decidir si
// aún está dentro de la prueba. NO aplica al plan de clínica (ver
// ClinicBaseIncludedDoctors) — ahí el cobro arranca de inmediato.
const TrialDuration = 14 * 24 * time.Hour

// ClinicBaseIncludedDoctors: cuántos doctores cubre el precio base de la
// clínica (STRIPE_CLINIC_BASE_PRICE_ID) antes de empezar a cobrar el
// extra por cabeza (STRIPE_CLINIC_EXTRA_DOCTOR_PRICE_ID). Decisión de
// producto: $5,000 MXN/mes cubre hasta 5 doctores, $1,000 MXN/mes por
// cada uno adicional.
const ClinicBaseIncludedDoctors = 5

type Config struct {
	SecretKey     string
	PriceID       string
	WebhookSecret string

	// Precios del plan de clínica — ver ClinicBaseIncludedDoctors. Si
	// cualquiera de las dos queda vacía, la clínica se comporta como una
	// integración no configurada (mismo criterio que el resto de
	// IsConfigured()): las rutas de clínica devuelven 503 en vez de
	// fallar de forma confusa.
	ClinicBasePriceID  string
	ClinicExtraPriceID string
}

func LoadConfigFromEnv() Config {
	return Config{
		SecretKey:          os.Getenv("STRIPE_SECRET_KEY"),
		PriceID:            os.Getenv("STRIPE_PRICE_ID"),
		WebhookSecret:      os.Getenv("STRIPE_WEBHOOK_SECRET"),
		ClinicBasePriceID:  os.Getenv("STRIPE_CLINIC_BASE_PRICE_ID"),
		ClinicExtraPriceID: os.Getenv("STRIPE_CLINIC_EXTRA_DOCTOR_PRICE_ID"),
	}
}

func (c Config) IsConfigured() bool {
	return c.SecretKey != "" && c.PriceID != ""
}

// IsClinicConfigured es igual que IsConfigured pero además exige las dos
// llaves de precio del plan de clínica — se piden aparte porque un doctor
// puede seguir suscribiéndose individualmente sin que el plan de clínica
// esté dado de alta todavía.
func (c Config) IsClinicConfigured() bool {
	return c.IsConfigured() && c.ClinicBasePriceID != "" && c.ClinicExtraPriceID != ""
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

// ClinicCheckoutParams son los datos necesarios para armar el Checkout del
// plan base de una clínica nueva. Solo lleva el concepto fijo (el dueño
// arranca solo, sin doctores extra) — el concepto de "doctor extra" se
// agrega después, la primera vez que la clínica cruza
// ClinicBaseIncludedDoctors (ver SetClinicExtraDoctorQuantity).
type ClinicCheckoutParams struct {
	ClinicID      uint
	CustomerEmail string
	SuccessURL    string
	CancelURL     string
}

// Client abstrae las llamadas a la API de Stripe que necesita la app:
// Checkout (para suscribirse, doctor individual o clínica), Customer
// Portal (gestionar/cancelar sin construir esa UI nosotros), y el manejo
// del segundo concepto de cobro de una clínica (doctores que sobrepasan
// el plan base) y la cancelación de una suscripción individual cuando un
// doctor se une a una clínica. Permite mockear en tests de integración
// sin llamar a la red real.
type Client interface {
	CreateCheckoutSession(ctx context.Context, params CheckoutParams) (checkoutURL string, err error)
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (portalURL string, err error)
	CreateClinicCheckoutSession(ctx context.Context, params ClinicCheckoutParams) (checkoutURL string, err error)
	// SetClinicExtraDoctorQuantity crea/actualiza/borra el concepto de
	// "doctor extra" de la clínica según cuántos sobrepasan el plan base.
	// extraItemID viene vacío si ese concepto todavía no existe en Stripe
	// (clínica con ClinicBaseIncludedDoctors o menos desde siempre).
	// Devuelve el ID del concepto (vacío si quantity llegó a 0 y se borró).
	SetClinicExtraDoctorQuantity(ctx context.Context, subscriptionID, extraItemID string, quantity int64) (newExtraItemID string, err error)
	// CancelSubscription cancela de inmediato (no al final del periodo) la
	// suscripción individual de un doctor que se acaba de unir a una
	// clínica — evita que le sigan cobrando dos veces.
	CancelSubscription(ctx context.Context, subscriptionID string) error
	// GetSubscriptionPeriodEnd consulta a Stripe cuándo se cobra la
	// próxima renovación de una suscripción activa. A diferencia de
	// TrialEndsAt (que sí se guarda en la base de datos porque es fijo
	// desde el alta), esta fecha solo vive en Stripe y puede recorrerse
	// con cada pago, así que se consulta en vivo en vez de guardarla.
	GetSubscriptionPeriodEnd(ctx context.Context, subscriptionID string) (time.Time, error)
}

type realClient struct {
	priceID            string
	clinicBasePriceID  string
	clinicExtraPriceID string
}

// NewClient construye el cliente real de Stripe. Solo se debe llamar si
// cfg.IsConfigured() es true. ClinicBasePriceID/ClinicExtraPriceID pueden
// venir vacías si el plan de clínica todavía no está dado de alta — los
// métodos de clínica simplemente fallan si se llaman sin esas dos
// configuradas (ver handlers, que ya revisan cfg.IsClinicConfigured()
// antes de intentarlo).
func NewClient(cfg Config) Client {
	stripe.Key = cfg.SecretKey
	return &realClient{
		priceID:            cfg.PriceID,
		clinicBasePriceID:  cfg.ClinicBasePriceID,
		clinicExtraPriceID: cfg.ClinicExtraPriceID,
	}
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

// clinicClientReferencePrefix distingue, en el webhook de
// checkout.session.completed, si el ClientReferenceID es el de un doctor
// individual (solo el número) o el de una clínica ("clinic:<id>") — así
// un solo endpoint de webhook sirve para los dos flujos sin necesitar dos
// rutas distintas.
const clinicClientReferencePrefix = "clinic:"

// ClinicClientReferenceID arma el ClientReferenceID que identifica a una
// clínica en el Checkout — exportado para que el webhook (en
// internal/handlers) sepa cómo reconocerlo y separarlo del de un doctor.
func ClinicClientReferenceID(clinicID uint) string {
	return clinicClientReferencePrefix + strconv.FormatUint(uint64(clinicID), 10)
}

// ParseClinicClientReferenceID es el inverso: si ref viene con el prefijo
// de clínica, devuelve su ID y ok=true; si es la referencia plana de un
// doctor individual, ok=false.
func ParseClinicClientReferenceID(ref string) (clinicID uint, ok bool) {
	if !strings.HasPrefix(ref, clinicClientReferencePrefix) {
		return 0, false
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(ref, clinicClientReferencePrefix), 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

func (r *realClient) CreateClinicCheckoutSession(ctx context.Context, params ClinicCheckoutParams) (string, error) {
	sParams := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:        stripe.String(params.SuccessURL),
		CancelURL:         stripe.String(params.CancelURL),
		ClientReferenceID: stripe.String(ClinicClientReferenceID(params.ClinicID)),
		CustomerEmail:     stripe.String(params.CustomerEmail),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(r.clinicBasePriceID), Quantity: stripe.Int64(1)},
		},
	}
	sParams.Context = ctx

	sess, err := checkoutsession.New(sParams)
	if err != nil {
		return "", err
	}
	return sess.URL, nil
}

func (r *realClient) SetClinicExtraDoctorQuantity(ctx context.Context, subscriptionID, extraItemID string, quantity int64) (string, error) {
	if quantity <= 0 {
		if extraItemID == "" {
			return "", nil // ya estaba en 0, nada que hacer
		}
		delParams := &stripe.SubscriptionItemParams{}
		delParams.Context = ctx
		if _, err := subscriptionitem.Del(extraItemID, delParams); err != nil {
			return "", err
		}
		return "", nil
	}

	if extraItemID == "" {
		newParams := &stripe.SubscriptionItemParams{
			Subscription: stripe.String(subscriptionID),
			Price:        stripe.String(r.clinicExtraPriceID),
			Quantity:     stripe.Int64(quantity),
		}
		newParams.Context = ctx
		item, err := subscriptionitem.New(newParams)
		if err != nil {
			return "", err
		}
		return item.ID, nil
	}

	updParams := &stripe.SubscriptionItemParams{Quantity: stripe.Int64(quantity)}
	updParams.Context = ctx
	if _, err := subscriptionitem.Update(extraItemID, updParams); err != nil {
		return "", err
	}
	return extraItemID, nil
}

func (r *realClient) CancelSubscription(ctx context.Context, subscriptionID string) error {
	cancelParams := &stripe.SubscriptionCancelParams{}
	cancelParams.Context = ctx
	_, err := subscription.Cancel(subscriptionID, cancelParams)
	return err
}

func (r *realClient) GetSubscriptionPeriodEnd(ctx context.Context, subscriptionID string) (time.Time, error) {
	params := &stripe.SubscriptionParams{}
	params.Context = ctx
	sub, err := subscription.Get(subscriptionID, params)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sub.CurrentPeriodEnd, 0).UTC(), nil
}
