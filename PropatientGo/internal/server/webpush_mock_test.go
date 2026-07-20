package server_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"propatient-api/internal/webpush"
)

// mockWebpushClient guarda cada notificación mandada, sin tocar un
// proveedor push real — mismo patrón que mockWhatsAppClient.
type mockWebpushClient struct {
	mu    sync.Mutex
	calls []string // endpoints a los que se mandó
}

func newMockWebpushClient() *mockWebpushClient {
	return &mockWebpushClient{}
}

func (m *mockWebpushClient) SendNotification(ctx context.Context, sub webpush.Subscription, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, sub.Endpoint)
	return nil
}

func (m *mockWebpushClient) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockWebpushClient) callsTo(endpoint string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, c := range m.calls {
		if c == endpoint {
			count++
		}
	}
	return count
}

// waitForCallsTo espera (con sondeo corto) a que "endpoint" tenga al menos
// "want" llamadas registradas — el envío va en una goroutine en segundo
// plano, igual que el de WhatsApp (ver whatsapp_mock_test.go).
func (m *mockWebpushClient) waitForCallsTo(t *testing.T, endpoint string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if m.callsTo(endpoint) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("tras 2s, %s solo tiene %d llamadas registradas, se esperaban al menos %d", endpoint, m.callsTo(endpoint), want)
}
