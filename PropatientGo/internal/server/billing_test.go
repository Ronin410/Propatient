package server_test

import (
	"net/http"
	"testing"
	"time"

	"propatient-api/internal/billing"
	"propatient-api/internal/geocoding"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/models"
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
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), billingConfig, nil, geocoding.NewClient(), nil, nil)

	req := map[string]any{"type": "customer.subscription.updated"}
	w := doRequest(t, router, http.MethodPost, "/api/billing/webhook", "", req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestBilling_ActiveSubscription_IncludesCurrentPeriodEnd confirma que,
// con una suscripción activa, /billing/status trae la fecha real de la
// próxima renovación consultada a Stripe (ver
// billing.Client.GetSubscriptionPeriodEnd) — no solo el status "active" a
// secas, que no le dice al doctor cuándo es su próximo cobro.
func TestBilling_ActiveSubscription_IncludesCurrentPeriodEnd(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	expectedEnd := time.Now().UTC().Add(21 * 24 * time.Hour).Truncate(time.Second)
	mockBilling.periodEnd = expectedEnd
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), billing.Config{}, mockBilling, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_period_end", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"subscription_status":    "active",
		"stripe_subscription_id": "sub_test_123",
	}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/billing/status", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, "active", body["subscriptionStatus"])
	require.NotNil(t, body["currentPeriodEnd"])
	got, err := time.Parse(time.RFC3339, body["currentPeriodEnd"].(string))
	require.NoError(t, err)
	assert.WithinDuration(t, expectedEnd, got, time.Second)
}

// TestBilling_TrialingSubscription_OmitsCurrentPeriodEnd confirma que un
// doctor en prueba (sin suscripción activa todavía) no trae
// currentPeriodEnd — esa fecha solo existe una vez que Stripe está
// cobrando de verdad.
func TestBilling_TrialingSubscription_OmitsCurrentPeriodEnd(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_still_trial", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/billing/status", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	assert.Equal(t, "trialing", body["subscriptionStatus"])
	assert.Nil(t, body["currentPeriodEnd"])
}

// TestBilling_GetStatus_ClinicDoctor_ReturnsManagedByClinic confirma que
// un doctor de clínica ve una respuesta distinta (managedByClinic: true,
// sin los demás campos de suscripción individual) — su acceso depende de
// la suscripción de la clínica, no de la suya propia.
func TestBilling_GetStatus_ClinicDoctor_ReturnsManagedByClinic(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_clinic_status", "password123")
	clinic := models.Clinic{Name: "Clínica de prueba", OwnerDoctorID: doc.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&doc).Update("clinic_id", clinic.ID).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/billing/status", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, true, body["managedByClinic"])
	assert.Nil(t, body["subscriptionStatus"])
}

// TestBilling_CreateCheckoutSession_ClinicDoctor_Returns409 confirma el
// punto central de este cambio: un doctor de clínica no puede arrancar una
// segunda suscripción individual encima de la de su clínica.
func TestBilling_CreateCheckoutSession_ClinicDoctor_Returns409(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mock := newMockBillingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), clinicBillingConfig(), mock, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_clinic_checkout", "password123")
	clinic := models.Clinic{Name: "Clínica de prueba", OwnerDoctorID: doc.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&doc).Update("clinic_id", clinic.ID).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/billing/checkout", token, nil)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

// TestBilling_CreatePortalSession_ClinicDoctor_Returns409 es el mismo
// criterio que el checkout, pero para el portal de facturación individual.
func TestBilling_CreatePortalSession_ClinicDoctor_Returns409(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mock := newMockBillingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), clinicBillingConfig(), mock, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_clinic_portal", "password123")
	clinic := models.Clinic{Name: "Clínica de prueba", OwnerDoctorID: doc.ID, SubscriptionStatus: "active"}
	require.NoError(t, db.Create(&clinic).Error)
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"clinic_id":          clinic.ID,
		"stripe_customer_id": "cus_old_individual_123",
	}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/billing/portal", token, nil)
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

// TestBilling_PastDueWithinGrace_AllowsAccess confirma el periodo de
// gracia: un cobro fallido reciente (past_due_since dentro de las últimas
// billing.PastDuePaymentGraceDuration) todavía deja pasar al doctor, en
// vez de bloquearlo de inmediato en el primer intento fallido.
func TestBilling_PastDueWithinGrace_AllowsAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_pastdue_grace", "password123")
	recentFailure := time.Now().UTC().Add(-2 * 24 * time.Hour)
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"subscription_status": "past_due",
		"past_due_since":      recentFailure,
	}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBilling_PastDueGraceExpired_BlocksAccess confirma el otro lado: una
// vez que pasó el periodo de gracia completo sin resolverse el pago, sí
// bloquea con 402 como cualquier suscripción vencida.
func TestBilling_PastDueGraceExpired_BlocksAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_pastdue_expired", "password123")
	oldFailure := time.Now().UTC().Add(-(billing.PastDuePaymentGraceDuration + 24*time.Hour))
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"subscription_status": "past_due",
		"past_due_since":      oldFailure,
	}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusPaymentRequired, w.Code)
}

// TestBilling_PastDueWithoutSince_BlocksAccess confirma el caso más
// seguro: si por lo que sea nunca se registró desde cuándo empezó el
// past_due (dato de antes de que este campo existiera, o un caso raro),
// se trata como fuera de gracia — bloquea en vez de dejar pasar
// indefinidamente.
func TestBilling_PastDueWithoutSince_BlocksAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_pastdue_nosince", "password123")
	require.NoError(t, db.Model(&doc).Update("subscription_status", "past_due").Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)
	assert.Equal(t, http.StatusPaymentRequired, w.Code)
}

// TestBilling_GetStatus_PastDue_IncludesGraceDeadline confirma que
// /billing/status expone hasta cuándo sigue el periodo de gracia, para que
// el frontend le muestre al doctor cuántos días le quedan antes de perder
// acceso.
func TestBilling_GetStatus_PastDue_IncludesGraceDeadline(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_pastdue_status", "password123")
	since := time.Now().UTC().Add(-2 * 24 * time.Hour)
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"subscription_status": "past_due",
		"past_due_since":      since,
	}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/billing/status", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, "past_due", body["subscriptionStatus"])
	require.NotNil(t, body["pastDueGraceEndsAt"])
	got, err := time.Parse(time.RFC3339, body["pastDueGraceEndsAt"].(string))
	require.NoError(t, err)
	assert.WithinDuration(t, since.Add(billing.PastDuePaymentGraceDuration), got, time.Second)
}

// TestBilling_CreateCheckoutSession_UsesLaunchPrice_WhenPromoActive
// confirma el punto central del precio de lanzamiento: mientras la fecha
// límite (STRIPE_LAUNCH_PRICE_ENDS_AT) no haya pasado, un doctor que se
// suscribe por primera vez debe ir con el Price de lanzamiento, no el
// regular (ver billing.Config.CheckoutPriceID).
func TestBilling_CreateCheckoutSession_UsesLaunchPrice_WhenPromoActive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	launchEndsAt := time.Now().UTC().Add(48 * time.Hour)
	cfg := billing.Config{
		SecretKey:         "sk_test_mock",
		PriceID:           "price_regular_mock",
		LaunchPriceID:     "price_launch_mock",
		LaunchPriceEndsAt: &launchEndsAt,
	}
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), cfg, mockBilling, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_launch_active", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/billing/checkout", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	call, ok := mockBilling.lastCheckoutCall()
	require.True(t, ok)
	assert.Equal(t, "price_launch_mock", call.PriceID)
}

// TestBilling_CreateCheckoutSession_UsesRegularPrice_WhenPromoExpired
// confirma el otro lado: pasada la fecha límite, un doctor NUEVO va con el
// precio regular — a quien ya se suscribió con el de lanzamiento no le
// cambia nada (Stripe no reajusta suscripciones ya activas solo porque
// cambie cuál es el Price "actual").
func TestBilling_CreateCheckoutSession_UsesRegularPrice_WhenPromoExpired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	launchEndedAt := time.Now().UTC().Add(-48 * time.Hour)
	cfg := billing.Config{
		SecretKey:         "sk_test_mock",
		PriceID:           "price_regular_mock",
		LaunchPriceID:     "price_launch_mock",
		LaunchPriceEndsAt: &launchEndedAt,
	}
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), cfg, mockBilling, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_launch_expired", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/billing/checkout", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	call, ok := mockBilling.lastCheckoutCall()
	require.True(t, ok)
	assert.Equal(t, "price_regular_mock", call.PriceID)
}

// TestBilling_CreateCheckoutSession_UsesRegularPrice_WhenLaunchNotConfigured
// confirma que sin STRIPE_LAUNCH_PRICE_ID/STRIPE_LAUNCH_PRICE_ENDS_AT
// definidas, no hay ninguna promoción — todos pagan el precio regular
// desde el primer día, mismo comportamiento que antes de que existiera
// esta función.
func TestBilling_CreateCheckoutSession_UsesRegularPrice_WhenLaunchNotConfigured(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	cfg := billing.Config{SecretKey: "sk_test_mock", PriceID: "price_regular_mock"}
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), cfg, mockBilling, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_no_launch", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/billing/checkout", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	call, ok := mockBilling.lastCheckoutCall()
	require.True(t, ok)
	assert.Equal(t, "price_regular_mock", call.PriceID)
}

// TestBilling_GetStatus_IncludesLaunchPromoInfo confirma que /billing/status
// le manda al frontend lo necesario para mostrar el descuento ANTES de que
// el doctor pague: si la promo sigue activa y los dos montos informativos.
func TestBilling_GetStatus_IncludesLaunchPromoInfo(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	launchEndsAt := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	cfg := billing.Config{
		SecretKey:         "sk_test_mock",
		PriceID:           "price_regular_mock",
		LaunchPriceID:     "price_launch_mock",
		LaunchPriceEndsAt: &launchEndsAt,
	}
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), cfg, mockBilling, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_launch_status", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/billing/status", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, true, body["launchPromoActive"])
	assert.Equal(t, float64(billing.IndividualLaunchPriceMXN), body["launchPriceMXN"])
	assert.Equal(t, float64(billing.IndividualRegularPriceMXN), body["regularPriceMXN"])
	require.NotNil(t, body["launchPromoEndsAt"])
	got, err := time.Parse(time.RFC3339, body["launchPromoEndsAt"].(string))
	require.NoError(t, err)
	assert.WithinDuration(t, launchEndsAt, got, time.Second)
}

// TestBilling_GetStatus_LaunchPromoInactive_WhenExpired confirma que,
// pasada la fecha límite, /billing/status ya no marca la promo como
// activa (aunque las variables sigan configuradas) — el frontend no debe
// seguir ofreciendo el precio de lanzamiento a quien todavía no se
// suscribe.
func TestBilling_GetStatus_LaunchPromoInactive_WhenExpired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockBilling := newMockBillingClient()
	launchEndedAt := time.Now().UTC().Add(-48 * time.Hour)
	cfg := billing.Config{
		SecretKey:         "sk_test_mock",
		PriceID:           "price_regular_mock",
		LaunchPriceID:     "price_launch_mock",
		LaunchPriceEndsAt: &launchEndedAt,
	}
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mustLocalStorage(t), cfg, mockBilling, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_billing_launch_status_expired", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/billing/status", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, false, body["launchPromoActive"])
}

func mustLocalStorage(t *testing.T) storage.Client {
	t.Helper()
	client, err := storage.NewClient(t.Context(), storage.Config{})
	require.NoError(t, err)
	return client
}
