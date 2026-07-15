package server_test

import (
	"context"
	"sync"
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
