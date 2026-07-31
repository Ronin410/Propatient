package handlers

import "testing"

// TestBilledExtraDoctors_ReactiveOnly confirma el comportamiento de
// siempre (sin capacidad reservada, reservedTotal=0): el cobro extra es
// exactamente "doctores actuales - 5", nunca menos que 0.
func TestBilledExtraDoctors_ReactiveOnly(t *testing.T) {
	cases := []struct {
		name          string
		actualCount   int64
		reservedTotal int
		wantExtra     int64
	}{
		{"5 doctores, sin extra", 5, 0, 0},
		{"menos de 5, sin extra", 3, 0, 0},
		{"7 doctores, 2 extra", 7, 0, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := billedExtraDoctors(tc.actualCount, tc.reservedTotal)
			if got != tc.wantExtra {
				t.Errorf("billedExtraDoctors(%d, %d) = %d, want %d", tc.actualCount, tc.reservedTotal, got, tc.wantExtra)
			}
		})
	}
}

// TestBilledExtraDoctors_ReservedCapacity_ActsAsFloor confirma el punto
// central de la nueva feature: la capacidad reservada nunca baja el
// cobro por debajo de lo reservado, y nunca lo sube más allá de lo que en
// verdad hay si eso es mayor a lo reservado.
func TestBilledExtraDoctors_ReservedCapacity_ActsAsFloor(t *testing.T) {
	cases := []struct {
		name          string
		actualCount   int64
		reservedTotal int
		wantExtra     int64
	}{
		{"reserva 10 sin invitar a nadie todavía (solo el dueño)", 1, 10, 5},
		{"reserva 10, ya hay 6 (el real es menor a lo reservado)", 6, 10, 5},
		{"reserva 10, ya hay 11 (el real manda porque es mayor)", 11, 10, 6},
		{"reserva igual al conteo real", 7, 7, 2},
		{"reserva por debajo del incluido en el plan base no hace nada", 5, 3, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := billedExtraDoctors(tc.actualCount, tc.reservedTotal)
			if got != tc.wantExtra {
				t.Errorf("billedExtraDoctors(%d, %d) = %d, want %d", tc.actualCount, tc.reservedTotal, got, tc.wantExtra)
			}
		})
	}
}
