package geocoding

import "testing"

func TestNewClient_SelectsProviderByEnv(t *testing.T) {
	t.Run("sin LOCATIONIQ_API_KEY usa Nominatim", func(t *testing.T) {
		t.Setenv("LOCATIONIQ_API_KEY", "")
		client := NewClient()
		if _, ok := client.(*nominatimClient); !ok {
			t.Fatalf("esperaba *nominatimClient, obtuvo %T", client)
		}
	})

	t.Run("con LOCATIONIQ_API_KEY usa LocationIQ", func(t *testing.T) {
		t.Setenv("LOCATIONIQ_API_KEY", "test-key")
		client := NewClient()
		liq, ok := client.(*locationIQClient)
		if !ok {
			t.Fatalf("esperaba *locationIQClient, obtuvo %T", client)
		}
		if liq.apiKey != "test-key" {
			t.Fatalf("esperaba apiKey 'test-key', obtuvo %q", liq.apiKey)
		}
	})
}
