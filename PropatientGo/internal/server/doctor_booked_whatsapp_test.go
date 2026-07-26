package server_test

import (
	"context"
	"net/http"
	"testing"

	"propatient-api/internal/billing"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/server"
	"propatient-api/internal/storage"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/require"
)

// TestCreateAppointment_DoctorBooked_NotifiesPatientWithUploadLink confirma
// que, cuando el DOCTOR agenda la cita (a diferencia de una solicitud
// pública), el paciente ahora sí recibe un WhatsApp de confirmación con el
// link de "sube tus documentos antes de la cita" — antes de este cambio,
// CreateAppointment no mandaba ningún aviso al paciente. El mensaje debe
// ser el mismo que el de "cita pública confirmada" (misma función,
// sendAppointmentDecisionWhatsApp) y no uno propio: una plantilla de
// Twilio nueva y sin configurar en producción caía a texto libre, que
// WhatsApp puede rechazar en silencio para un paciente que nunca le
// escribió antes al número del negocio.
func TestCreateAppointment_DoctorBooked_NotifiesPatientWithUploadLink(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	wa := newMockWhatsAppClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), wa, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_wa_doctor_booked", "password123")
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/appointments", docToken, map[string]any{
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"service":             "Consulta general",
		"patientFirstName":    "Lucía",
		"patientLastName":     "Reyes",
		"patientPhone":        "5557778888",
		"patientEmail":        "lucia.wa@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	wa.waitForCallsTo(t, "5557778888", 1)

	appt := decodeJSON(t, w)
	apptID := int(appt["id"].(float64))

	var uploadToken string
	require.NoError(t, db.Raw("SELECT upload_token FROM appointments WHERE id = ?", apptID).Scan(&uploadToken).Error)
	require.NotEmpty(t, uploadToken, "CreateAppointment debe generar el upload_token para poder mandarlo en el WhatsApp")

	body := wa.lastBodyTo(t, "5557778888")
	require.Contains(t, body, "quedó confirmada", "debe usar el mismo mensaje que la confirmación de una cita pública")
	require.Contains(t, body, "/public-upload/"+uploadToken)
}
