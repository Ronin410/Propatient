package server_test

import (
	"net/http"
	"testing"
	"time"

	"propatient-api/internal/billing"
	"propatient-api/internal/geocoding"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/server"
	"propatient-api/internal/storage"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBilling_TrialActive_AllowsAccess confirma el caso normal: un doctor
// recién creado, dentro de su periodo de prueba, puede usar la app.
func TestBilling_TrialActive_AllowsAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_trial_ok", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBilling_TrialExpired_BlocksAccess es el caso central: sin prueba ni
// suscripción activa, la API responde 402 en vez de dejar pasar la petición.
func TestBilling_TrialExpired_BlocksAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_trial_expired", "password123")
	expired := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&doc).Update("trial_ends_at", expired).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusPaymentRequired, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "subscription_required", body["error"])
}

// TestBilling_ActiveSubscription_AllowsAccessEvenAfterTrial confirma que
// una suscripción activa reemplaza a la prueba, aunque ya haya vencido.
func TestBilling_ActiveSubscription_AllowsAccessEvenAfterTrial(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_active", "password123")
	expired := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"trial_ends_at":       expired,
		"subscription_status": "active",
	}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBilling_ExpiredTrial_CanStillReachCheckout verifica que las rutas de
// facturación NUNCA queden atrapadas detrás de su propio candado: un
// doctor con la prueba vencida tiene que poder llegar a pagar. Sin Stripe
// configurado en el test, el resultado esperado es 503 (no configurado),
// nunca 402 (bloqueado) — eso probaría que el gate sí las está alcanzando.
func TestBilling_ExpiredTrial_CanStillReachCheckout(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_checkout", "password123")
	expired := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&doc).Update("trial_ends_at", expired).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/billing/status", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "trialing", body["subscriptionStatus"])

	w = doRequest(t, router, http.MethodPost, "/api/billing/checkout", token, nil)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "sin Stripe configurado debe ser 503, no 402")
}

// TestBilling_StaffSharesDoctorsSubscriptionStatus confirma que el
// personal queda bloqueado igual que el doctor cuando la prueba vence: no
// hay forma de seguir usando la app "por detrás" con una cuenta de personal.
func TestBilling_StaffSharesDoctorsSubscriptionStatus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_staff", "password123")
	expired := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&doc).Update("trial_ends_at", expired).Error)

	staff := testutil.CreateTestStaff(t, db, doc.ID, "personal@billing-test.local", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodGet, "/api/patients", staffToken, nil)
	assert.Equal(t, http.StatusPaymentRequired, w.Code)
}

// TestBilling_StatusEndpoint_IsDoctorOnly confirma que el personal no
// puede consultar ni gestionar la facturación del consultorio.
func TestBilling_StatusEndpoint_IsDoctorOnly(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_staffonly", "password123")
	staff := testutil.CreateTestStaff(t, db, doc.ID, "personal2@billing-test.local", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodGet, "/api/billing/status", staffToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestBilling_WebhookRejectsBadSignature confirma que el webhook público
// no acepta peticiones sin una firma Stripe-Signature válida (cualquiera
// podría intentar mandar un payload falso marcando una suscripción como
// activa si no se verificara la firma).
func TestBilling_WebhookRejectsBadSignature(t *testing.T) {
	db := testutil.SetupTestDB(t)
	billingConfig := billing.Config{WebhookSecret: "whsec_test_secret"}
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), billingConfig, nil, geocoding.NewClient(), nil)

	req := map[string]any{"type": "customer.subscription.updated"}
	w := doRequest(t, router, http.MethodPost, "/api/billing/webhook", "", req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func mustLocalStorage(t *testing.T) storage.Client {
	t.Helper()
	client, err := storage.NewClient(t.Context(), storage.Config{})
	require.NoError(t, err)
	return client
}
