package server_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"propatient-api/internal/billing"
)

// mockBillingClient implementa billing.Client sin llamar a Stripe de
// verdad — registra cada llamada para que los tests de clínica puedan
// verificar qué se le pidió (cancelar suscripción individual, ajustar
// cantidad de doctores extra) sin depender de credenciales reales.
type mockBillingClient struct {
	mu sync.Mutex

	clinicCheckoutCalls   []billing.ClinicCheckoutParams
	canceledSubscriptions []string
	extraQtyCalls         []mockExtraQtyCall
	nextExtraItemSeq      int
	// periodEnd es lo que devuelve GetSubscriptionPeriodEnd; por defecto
	// (zero value) simula 30 días a partir de ahora, como cualquier
	// suscripción real recién pagada.
	periodEnd time.Time
}

type mockExtraQtyCall struct {
	SubscriptionID string
	ExtraItemID    string
	Quantity       int64
}

func newMockBillingClient() *mockBillingClient {
	return &mockBillingClient{}
}

func (m *mockBillingClient) CreateCheckoutSession(ctx context.Context, params billing.CheckoutParams) (string, error) {
	return "https://checkout.stripe.com/mock/doctor", nil
}

func (m *mockBillingClient) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	return "https://billing.stripe.com/mock/portal", nil
}

func (m *mockBillingClient) CreateClinicCheckoutSession(ctx context.Context, params billing.ClinicCheckoutParams) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clinicCheckoutCalls = append(m.clinicCheckoutCalls, params)
	return "https://checkout.stripe.com/mock/clinic", nil
}

func (m *mockBillingClient) SetClinicExtraDoctorQuantity(ctx context.Context, subscriptionID, extraItemID string, quantity int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.extraQtyCalls = append(m.extraQtyCalls, mockExtraQtyCall{SubscriptionID: subscriptionID, ExtraItemID: extraItemID, Quantity: quantity})
	if quantity <= 0 {
		return "", nil
	}
	if extraItemID != "" {
		return extraItemID, nil
	}
	m.nextExtraItemSeq++
	return fmt.Sprintf("mock_extra_item_%d", m.nextExtraItemSeq), nil
}

func (m *mockBillingClient) CancelSubscription(ctx context.Context, subscriptionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.canceledSubscriptions = append(m.canceledSubscriptions, subscriptionID)
	return nil
}

func (m *mockBillingClient) GetSubscriptionPeriodEnd(ctx context.Context, subscriptionID string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.periodEnd.IsZero() {
		return time.Now().UTC().Add(30 * 24 * time.Hour), nil
	}
	return m.periodEnd, nil
}

func (m *mockBillingClient) wasCanceled(subscriptionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.canceledSubscriptions {
		if s == subscriptionID {
			return true
		}
	}
	return false
}

func (m *mockBillingClient) lastExtraQtyCall() (mockExtraQtyCall, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.extraQtyCalls) == 0 {
		return mockExtraQtyCall{}, false
	}
	return m.extraQtyCalls[len(m.extraQtyCalls)-1], true
}
