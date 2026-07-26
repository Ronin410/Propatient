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

// La firma de referencia se calculó de forma independiente (Python +
// hashlib, no con este mismo código Go) para no terminar validando la
// implementación contra sí misma — ver el algoritmo documentado por
// Twilio en https://www.twilio.com/docs/usage/security.
func TestValidateInboundSignature(t *testing.T) {
	const authToken = "testtoken123"
	const fullURL = "https://api.propatient.pro/api/whatsapp/webhook"
	params := map[string]string{
		"From":       "whatsapp:+525551234567",
		"Body":       "Hola doctor",
		"MessageSid": "SM123abc",
	}
	const validSignature = "squK9QIm/LgGPdoiu2LcMOoegNk="

	if !ValidateInboundSignature(authToken, fullURL, params, validSignature) {
		t.Error("una firma calculada correctamente debe validar")
	}
	if ValidateInboundSignature(authToken, fullURL, params, "firma-inventada") {
		t.Error("una firma incorrecta no debe validar")
	}
	if ValidateInboundSignature("otro-token", fullURL, params, validSignature) {
		t.Error("el auth token equivocado no debe validar")
	}
	if ValidateInboundSignature(authToken, "https://otra-url.com/webhook", params, validSignature) {
		t.Error("una URL distinta a la firmada no debe validar")
	}
	tamperedParams := map[string]string{"From": params["From"], "Body": "Otro texto", "MessageSid": params["MessageSid"]}
	if ValidateInboundSignature(authToken, fullURL, tamperedParams, validSignature) {
		t.Error("parámetros alterados no deben validar")
	}
	if ValidateInboundSignature(authToken, fullURL, params, "") {
		t.Error("una firma vacía nunca debe validar")
	}
	if ValidateInboundSignature("", fullURL, params, validSignature) {
		t.Error("sin auth token configurado nunca debe validar")
	}
}
