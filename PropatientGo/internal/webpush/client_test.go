package webpush

import "testing"

func TestConfig_IsConfigured(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"las tres presentes", Config{VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv", VAPIDSubject: "mailto:a@b.com"}, true},
		{"falta la pública", Config{VAPIDPrivateKey: "priv", VAPIDSubject: "mailto:a@b.com"}, false},
		{"falta la privada", Config{VAPIDPublicKey: "pub", VAPIDSubject: "mailto:a@b.com"}, false},
		{"falta el subject", Config{VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv"}, false},
		{"vacío", Config{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.IsConfigured(); got != tc.want {
				t.Errorf("IsConfigured() = %v, want %v", got, tc.want)
			}
		})
	}
}
