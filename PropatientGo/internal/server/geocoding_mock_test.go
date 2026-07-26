package server_test

import (
	"context"
	"sync"

	"propatient-api/internal/geocoding"
)

// mockGeocodingClient devuelve coordenadas fijas sin tocar la red — así los
// tests de "activar listado público" no dependen de Nominatim estando
// disponible ni le pegan a la API real en cada corrida.
type mockGeocodingClient struct {
	mu    sync.Mutex
	calls []string
	coord geocoding.Coordinates
}

func newMockGeocodingClient() *mockGeocodingClient {
	return &mockGeocodingClient{coord: geocoding.Coordinates{Latitude: 25.789, Longitude: -108.998}}
}

func (m *mockGeocodingClient) Geocode(ctx context.Context, address string) (*geocoding.Coordinates, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, address)
	coord := m.coord
	return &coord, nil
}

func (m *mockGeocodingClient) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}
