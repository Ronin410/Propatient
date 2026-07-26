package server_test

import (
	"context"
	"fmt"
	"sync"

	"propatient-api/internal/googlecalendar"
)

// mockCalendarClient registra las llamadas que le hacen los handlers, sin
// tocar la red — así se puede probar la integración con Google Calendar sin
// depender de credenciales reales.
type mockCalendarClient struct {
	mu sync.Mutex

	created []googlecalendar.EventInput
	updated []struct {
		EventID string
		Event   googlecalendar.EventInput
	}
	deleted []string

	nextEventID int
}

func newMockCalendarClient() *mockCalendarClient {
	return &mockCalendarClient{}
}

func (m *mockCalendarClient) CreateEvent(ctx context.Context, refreshToken string, ev googlecalendar.EventInput) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextEventID++
	id := fmt.Sprintf("mock-event-%d", m.nextEventID)
	m.created = append(m.created, ev)
	return id, nil
}

func (m *mockCalendarClient) UpdateEvent(ctx context.Context, refreshToken string, eventID string, ev googlecalendar.EventInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updated = append(m.updated, struct {
		EventID string
		Event   googlecalendar.EventInput
	}{eventID, ev})
	return nil
}

func (m *mockCalendarClient) DeleteEvent(ctx context.Context, refreshToken string, eventID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleted = append(m.deleted, eventID)
	return nil
}

func (m *mockCalendarClient) createdCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.created)
}

func (m *mockCalendarClient) updatedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.updated)
}

func (m *mockCalendarClient) deletedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.deleted)
}
