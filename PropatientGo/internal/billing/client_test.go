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

// TestConfig_IsClinicLaunchPromoActive es el equivalente de
// TestConfig_IsLaunchPromoActive para el plan de clínica: exige las DOS
// Price de lanzamiento de clínica configuradas (base y extra van
// juntas), además de la fecha límite compartida.
func TestConfig_IsClinicLaunchPromoActive(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"sin nada configurado", Config{}, false},
		{"solo base, sin extra", Config{ClinicLaunchBasePriceID: "price_base", LaunchPriceEndsAt: &future}, false},
		{"solo extra, sin base", Config{ClinicLaunchExtraPriceID: "price_extra", LaunchPriceEndsAt: &future}, false},
		{"ambas sin fecha", Config{ClinicLaunchBasePriceID: "price_base", ClinicLaunchExtraPriceID: "price_extra"}, false},
		{
			"todo configurado, fecha futura",
			Config{ClinicLaunchBasePriceID: "price_base", ClinicLaunchExtraPriceID: "price_extra", LaunchPriceEndsAt: &future},
			true,
		},
		{
			"todo configurado, fecha ya pasada",
			Config{ClinicLaunchBasePriceID: "price_base", ClinicLaunchExtraPriceID: "price_extra", LaunchPriceEndsAt: &past},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.IsClinicLaunchPromoActive(now))
		})
	}
}

// TestConfig_ClinicCheckoutPriceIDs confirma que los dos Price elegidos
// (base y extra) siguen exactamente a IsClinicLaunchPromoActive.
func TestConfig_ClinicCheckoutPriceIDs(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	active := Config{
		ClinicBasePriceID: "price_base_regular", ClinicExtraPriceID: "price_extra_regular",
		ClinicLaunchBasePriceID: "price_base_launch", ClinicLaunchExtraPriceID: "price_extra_launch",
		LaunchPriceEndsAt: &future,
	}
	assert.Equal(t, "price_base_launch", active.ClinicCheckoutBasePriceID(now))
	assert.Equal(t, "price_extra_launch", active.ClinicCheckoutExtraPriceID(now))

	expired := active
	expired.LaunchPriceEndsAt = &past
	assert.Equal(t, "price_base_regular", expired.ClinicCheckoutBasePriceID(now))
	assert.Equal(t, "price_extra_regular", expired.ClinicCheckoutExtraPriceID(now))

	unconfigured := Config{ClinicBasePriceID: "price_base_regular", ClinicExtraPriceID: "price_extra_regular"}
	assert.Equal(t, "price_base_regular", unconfigured.ClinicCheckoutBasePriceID(now))
	assert.Equal(t, "price_extra_regular", unconfigured.ClinicCheckoutExtraPriceID(now))
}
