package server_test

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockWhatsAppClient guarda cada mensaje mandado, sin tocar la API real de
// Twilio — permite verificar en los tests a quién se le mandó qué, sin
// depender de credenciales.
type mockWhatsAppClient struct {
	mu    sync.Mutex
	calls []mockWhatsAppCall
}

type mockWhatsAppCall struct {
	To   string
	Body string
}

func newMockWhatsAppClient() *mockWhatsAppClient {
	return &mockWhatsAppClient{}
}

func (m *mockWhatsAppClient) SendMessage(ctx context.Context, toPhone, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockWhatsAppCall{To: toPhone, Body: body})
	return nil
}

func (m *mockWhatsAppClient) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockWhatsAppClient) callsTo(phone string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, c := range m.calls {
		if c.To == phone {
			count++
		}
	}
	return count
}

// waitForCallsTo espera (con sondeo corto) a que "phone" tenga al menos
// "want" llamadas registradas. Los avisos de WhatsApp se mandan en una
// goroutine en segundo plano (para no bloquear la respuesta HTTP al
// paciente — ver el comentario en CreatePublicAppointment), así que un
// test no puede asumir que ya llegaron justo al recibir la respuesta.
func (m *mockWhatsAppClient) waitForCallsTo(t *testing.T, phone string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if m.callsTo(phone) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("tras 2s, %s solo tiene %d llamadas registradas, se esperaban al menos %d", phone, m.callsTo(phone), want)
}
