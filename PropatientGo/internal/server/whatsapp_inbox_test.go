package server_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"propatient-api/internal/billing"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/storage"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTwilioWebhookURL = "http://testserver/api/whatsapp/webhook"

// signTwilioParams calcula la firma tal como la mandaría Twilio de verdad
// (algoritmo independiente al de internal/whatsapp.ValidateInboundSignature,
// ya cubierto en su propio test) — se usa aquí solo para simular una
// petición legítima de Twilio en las pruebas de integración del webhook.
func signTwilioParams(authToken, fullURL string, params url.Values) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	buf.WriteString(fullURL)
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString(params.Get(k))
	}
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(buf.String()))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func postTwilioWebhook(router http.Handler, authToken string, params url.Values, signatureOverride *string) *httptest.ResponseRecorder {
	sig := signTwilioParams(authToken, testTwilioWebhookURL, params)
	if signatureOverride != nil {
		sig = *signatureOverride
	}
	req := httptest.NewRequest(http.MethodPost, "/api/whatsapp/webhook", strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Twilio-Signature", sig)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestTwilioWebhook_NotConfiguredRejects(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, nil)

	params := url.Values{"From": {"whatsapp:+525551234567"}, "Body": {"Hola"}, "MessageSid": {"SM1"}}
	w := postTwilioWebhook(router, "", params, nil)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestTwilioWebhook_RejectsInvalidSignature(t *testing.T) {
	t.Setenv("TWILIO_AUTH_TOKEN", "secret-token")
	t.Setenv("TWILIO_WEBHOOK_URL", testTwilioWebhookURL)

	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, nil)

	params := url.Values{"From": {"whatsapp:+525551234567"}, "Body": {"Hola"}, "MessageSid": {"SM1"}}
	badSig := "firma-invalida"
	w := postTwilioWebhook(router, "secret-token", params, &badSig)
	assert.Equal(t, http.StatusForbidden, w.Code)

	var count int64
	db.Model(&models.WhatsAppMessage{}).Count(&count)
	assert.Equal(t, int64(0), count, "una firma inválida no debe guardar nada")
}

func TestTwilioWebhook_ClassifiesPatientReplyWithDoctorContext(t *testing.T) {
	t.Setenv("TWILIO_AUTH_TOKEN", "secret-token")
	t.Setenv("TWILIO_WEBHOOK_URL", testTwilioWebhookURL)

	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_inbox_patient", "password123")
	patient := models.Patient{FirstName: "Ana", LastName: "López", Phone: "5559876543"}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Model(&doc).Association("Patients").Append(&patient))
	appt := models.Appointment{PatientID: patient.ID, DoctorID: doc.ID, Status: "PENDING"}
	require.NoError(t, db.Create(&appt).Error)

	params := url.Values{
		"From":       {"whatsapp:+5215559876543"}, // con el "1" extra que agrega WhatsApp para México
		"Body":       {"No puedo esa fecha"},
		"MessageSid": {"SM_patient_1"},
	}
	w := postTwilioWebhook(router, "secret-token", params, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var msg models.WhatsAppMessage
	require.NoError(t, db.Where("twilio_sid = ?", "SM_patient_1").First(&msg).Error)
	assert.Equal(t, "PATIENT", msg.Category)
	assert.Equal(t, "INBOUND", msg.Direction)
	assert.Equal(t, "No puedo esa fecha", msg.Body)
	require.NotNil(t, msg.PatientID)
	assert.Equal(t, patient.ID, *msg.PatientID)
	require.NotNil(t, msg.DoctorID)
	assert.Equal(t, doc.ID, *msg.DoctorID)
}

func TestTwilioWebhook_ClassifiesDoctorSupportMessage(t *testing.T) {
	t.Setenv("TWILIO_AUTH_TOKEN", "secret-token")
	t.Setenv("TWILIO_WEBHOOK_URL", testTwilioWebhookURL)

	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_inbox_support", "password123")
	require.NoError(t, db.Model(&doc).Update("phone", "5551239876").Error)

	params := url.Values{
		"From":       {"whatsapp:+525551239876"},
		"Body":       {"Necesito ayuda con mi suscripción"},
		"MessageSid": {"SM_doctor_1"},
	}
	w := postTwilioWebhook(router, "secret-token", params, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var msg models.WhatsAppMessage
	require.NoError(t, db.Where("twilio_sid = ?", "SM_doctor_1").First(&msg).Error)
	assert.Equal(t, "DOCTOR_SUPPORT", msg.Category)
	require.NotNil(t, msg.DoctorID)
	assert.Equal(t, doc.ID, *msg.DoctorID)
	assert.Nil(t, msg.PatientID)
}

func TestTwilioWebhook_UnknownNumberStillStoredForReview(t *testing.T) {
	t.Setenv("TWILIO_AUTH_TOKEN", "secret-token")
	t.Setenv("TWILIO_WEBHOOK_URL", testTwilioWebhookURL)

	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, nil)

	params := url.Values{
		"From":       {"whatsapp:+15555550123"},
		"Body":       {"Hola, quién es?"},
		"MessageSid": {"SM_unknown_1"},
	}
	w := postTwilioWebhook(router, "secret-token", params, nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var msg models.WhatsAppMessage
	require.NoError(t, db.Where("twilio_sid = ?", "SM_unknown_1").First(&msg).Error)
	assert.Equal(t, "PATIENT", msg.Category)
	assert.Nil(t, msg.DoctorID)
	assert.Nil(t, msg.PatientID)
}

func TestTwilioWebhook_IdempotentOnRetry(t *testing.T) {
	t.Setenv("TWILIO_AUTH_TOKEN", "secret-token")
	t.Setenv("TWILIO_WEBHOOK_URL", testTwilioWebhookURL)

	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, nil)

	params := url.Values{"From": {"whatsapp:+15555550199"}, "Body": {"Hola"}, "MessageSid": {"SM_retry_1"}}
	w1 := postTwilioWebhook(router, "secret-token", params, nil)
	assert.Equal(t, http.StatusOK, w1.Code)
	w2 := postTwilioWebhook(router, "secret-token", params, nil)
	assert.Equal(t, http.StatusOK, w2.Code)

	var count int64
	db.Model(&models.WhatsAppMessage{}).Where("twilio_sid = ?", "SM_retry_1").Count(&count)
	assert.Equal(t, int64(1), count, "el mismo MessageSid reintentado no debe duplicar el mensaje")
}

func TestAdminWhatsAppThreads_RequireSuperAdmin(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_no_inbox_access", "password123")
	doctorToken := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/admin/whatsapp/threads?category=PATIENT", doctorToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdminWhatsAppThreads_ListMessagesAndReply(t *testing.T) {
	t.Setenv("TWILIO_AUTH_TOKEN", "secret-token")
	t.Setenv("TWILIO_WEBHOOK_URL", testTwilioWebhookURL)

	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	wa := newMockWhatsAppClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), wa, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_inbox_reply", "password123")
	patient := models.Patient{FirstName: "Carlos", LastName: "Ruiz", Phone: "5557001122"}
	require.NoError(t, db.Create(&patient).Error)
	require.NoError(t, db.Model(&doc).Association("Patients").Append(&patient))
	appt := models.Appointment{PatientID: patient.ID, DoctorID: doc.ID, Status: "PENDING"}
	require.NoError(t, db.Create(&appt).Error)

	params := url.Values{"From": {"whatsapp:+525557001122"}, "Body": {"¿Puedo cambiar mi cita?"}, "MessageSid": {"SM_reply_flow"}}
	w := postTwilioWebhook(router, "secret-token", params, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	admin := testutil.CreateTestSuperAdmin(t, db, "admin_inbox", "clavesegura123")
	adminToken := testutil.TokenForSuperAdmin(t, admin.ID, admin.Username)

	w = doRequest(t, router, http.MethodGet, "/api/admin/whatsapp/threads?category=PATIENT", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var threads []map[string]any
	decodeJSONList(t, w, &threads)
	require.Len(t, threads, 1)
	assert.Equal(t, "+525557001122", threads[0]["phone"])
	assert.Equal(t, float64(1), threads[0]["unreadCount"])
	assert.Equal(t, "Carlos Ruiz", threads[0]["patientName"])

	w = doRequest(t, router, http.MethodGet, "/api/admin/whatsapp/threads/"+url.QueryEscape("+525557001122")+"/messages", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var msgs []map[string]any
	decodeJSONList(t, w, &msgs)
	require.Len(t, msgs, 1)

	// Abrir la conversación debe marcar el mensaje entrante como leído.
	w = doRequest(t, router, http.MethodGet, "/api/admin/whatsapp/threads?category=PATIENT", adminToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	decodeJSONList(t, w, &threads)
	assert.Equal(t, float64(0), threads[0]["unreadCount"])

	w = doRequest(t, router, http.MethodPost, "/api/admin/whatsapp/threads/"+url.QueryEscape("+525557001122")+"/messages", adminToken, map[string]any{
		"message": "Claro, ¿qué día te acomoda?",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	wa.waitForCallsTo(t, "+525557001122", 1)

	var outbound models.WhatsAppMessage
	require.NoError(t, db.Where("phone = ? AND direction = ?", "+525557001122", "OUTBOUND").First(&outbound).Error)
	assert.Equal(t, "PATIENT", outbound.Category)
	require.NotNil(t, outbound.PatientID)
	assert.Equal(t, patient.ID, *outbound.PatientID)
}
