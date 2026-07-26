package whatsapp

import "testing"

func TestNormalizeE164(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"local sin código de país asume México", "5512345678", "+525512345678"},
		{"ya trae +", "+15551234567", "+15551234567"},
		{"limpia espacios y guiones", "55 1234-5678", "+525512345678"},
		{"limpia espacios y guiones con +", "+1 (555) 123-4567", "+15551234567"},
		{"vacío da vacío", "", ""},
		{"solo espacios da vacío", "   ", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeE164(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeE164(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
