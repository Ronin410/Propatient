package billing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestConfig_IsLaunchPromoActive cubre las combinaciones de configuración
// del precio de lanzamiento: solo cuenta como activa si AMBAS variables
// están definidas Y la fecha límite todavía no pasó.
func TestConfig_IsLaunchPromoActive(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"sin nada configurado", Config{}, false},
		{"solo LaunchPriceID, sin fecha", Config{LaunchPriceID: "price_launch"}, false},
		{"solo fecha, sin LaunchPriceID", Config{LaunchPriceEndsAt: &future}, false},
		{"ambas configuradas, fecha futura", Config{LaunchPriceID: "price_launch", LaunchPriceEndsAt: &future}, true},
		{"ambas configuradas, fecha ya pasada", Config{LaunchPriceID: "price_launch", LaunchPriceEndsAt: &past}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.IsLaunchPromoActive(now))
		})
	}
}

// TestConfig_CheckoutPriceID confirma que el Price elegido sigue
// exactamente a IsLaunchPromoActive: de lanzamiento mientras esté activa,
// regular en cualquier otro caso.
func TestConfig_CheckoutPriceID(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	active := Config{PriceID: "price_regular", LaunchPriceID: "price_launch", LaunchPriceEndsAt: &future}
	assert.Equal(t, "price_launch", active.CheckoutPriceID(now))

	expired := Config{PriceID: "price_regular", LaunchPriceID: "price_launch", LaunchPriceEndsAt: &past}
	assert.Equal(t, "price_regular", expired.CheckoutPriceID(now))

	unconfigured := Config{PriceID: "price_regular"}
	assert.Equal(t, "price_regular", unconfigured.CheckoutPriceID(now))
}
