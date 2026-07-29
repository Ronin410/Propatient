package referral_test

import (
	"context"
	"testing"
	"time"

	"propatient-api/internal/billing"
	"propatient-api/internal/models"
	"propatient-api/internal/referral"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBillingClient implementa billing.Client sin llamar a Stripe de
// verdad, solo para verificar con qué parámetros se llamó
// ExtendSubscriptionByDays.
type mockBillingClient struct {
	extendCalls []mockExtendCall
}

type mockExtendCall struct {
	SubscriptionID string
	Days           int
}

func (m *mockBillingClient) CreateCheckoutSession(ctx context.Context, params billing.CheckoutParams) (string, error) {
	return "", nil
}
func (m *mockBillingClient) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	return "", nil
}
func (m *mockBillingClient) CreateClinicCheckoutSession(ctx context.Context, params billing.ClinicCheckoutParams) (string, error) {
	return "", nil
}
func (m *mockBillingClient) SetClinicExtraDoctorQuantity(ctx context.Context, subscriptionID, extraItemID, priceID string, quantity int64) (string, error) {
	return "", nil
}
func (m *mockBillingClient) CancelSubscription(ctx context.Context, subscriptionID string) error {
	return nil
}
func (m *mockBillingClient) GetSubscriptionPeriodEnd(ctx context.Context, subscriptionID string) (time.Time, error) {
	return time.Now().UTC().Add(30 * 24 * time.Hour), nil
}
func (m *mockBillingClient) ExtendSubscriptionByDays(ctx context.Context, subscriptionID string, days int) error {
	m.extendCalls = append(m.extendCalls, mockExtendCall{SubscriptionID: subscriptionID, Days: days})
	return nil
}

// TestGenerateCode_UniqueAndWellFormed confirma que el código generado
// respeta el alfabeto sin caracteres ambiguos y que dos llamadas
// consecutivas no chocan.
func TestGenerateCode_UniqueAndWellFormed(t *testing.T) {
	db := testutil.SetupTestDB(t)

	code1, err := referral.GenerateCode(db)
	require.NoError(t, err)
	assert.Len(t, code1, 8)
	for _, ch := range code1 {
		assert.NotContains(t, "01OI", string(ch), "el código no debe usar caracteres ambiguos")
	}

	// Se "reserva" el primer código asignándolo a un doctor de prueba,
	// para confirmar que la segunda llamada no lo vuelve a generar.
	doc := testutil.CreateTestDoctor(t, db, "doc_ref_gen", "password123")
	require.NoError(t, db.Model(&doc).Update("referral_code", code1).Error)

	code2, err := referral.GenerateCode(db)
	require.NoError(t, err)
	assert.NotEqual(t, code1, code2)
}

// TestApplyCodeIfValid_ValidCode confirma el camino feliz: un doctor
// recién registrado captura el código de otro y se crea una fila
// Referral en estado "pending".
func TestApplyCodeIfValid_ValidCode(t *testing.T) {
	db := testutil.SetupTestDB(t)

	referrer := testutil.CreateTestDoctor(t, db, "doc_ref_referrer1", "password123")
	require.NoError(t, db.Model(&referrer).Update("referral_code", "CODE0001").Error)
	referred := testutil.CreateTestDoctor(t, db, "doc_ref_referred1", "password123")

	applied := referral.ApplyCodeIfValid(db, referred.ID, "code0001")
	assert.True(t, applied, "debe aceptar el código sin importar mayúsculas/minúsculas")

	var ref models.Referral
	require.NoError(t, db.Where("referred_doctor_id = ?", referred.ID).First(&ref).Error)
	assert.Equal(t, referrer.ID, ref.ReferrerDoctorID)
	assert.Equal(t, "pending", ref.Status)
}

// TestApplyCodeIfValid_InvalidCode confirma que un código inexistente no
// crea nada y no truena.
func TestApplyCodeIfValid_InvalidCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	referred := testutil.CreateTestDoctor(t, db, "doc_ref_referred2", "password123")

	applied := referral.ApplyCodeIfValid(db, referred.ID, "NOEXISTE")
	assert.False(t, applied)

	var count int64
	db.Model(&models.Referral{}).Count(&count)
	assert.Zero(t, count)
}

// TestApplyCodeIfValid_EmptyCode confirma que un código vacío (el campo
// opcional del registro, sin capturar nada) simplemente no hace nada.
func TestApplyCodeIfValid_EmptyCode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	referred := testutil.CreateTestDoctor(t, db, "doc_ref_referred3", "password123")

	applied := referral.ApplyCodeIfValid(db, referred.ID, "   ")
	assert.False(t, applied)
}

// TestApplyCodeIfValid_SelfReferral confirma que un doctor no puede
// usar su propio código para auto-referirse.
func TestApplyCodeIfValid_SelfReferral(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_ref_self", "password123")
	require.NoError(t, db.Model(&doc).Update("referral_code", "SELFCODE").Error)

	applied := referral.ApplyCodeIfValid(db, doc.ID, "SELFCODE")
	assert.False(t, applied)
}

// TestApplyCodeIfValid_AtCap confirma que, tras 10 referencias contra el
// mismo referidor, un intento adicional se rechaza en silencio (mismo
// resultado que un código inválido, a propósito).
func TestApplyCodeIfValid_AtCap(t *testing.T) {
	db := testutil.SetupTestDB(t)
	referrer := testutil.CreateTestDoctor(t, db, "doc_ref_capped", "password123")
	require.NoError(t, db.Model(&referrer).Update("referral_code", "CAPCODE1").Error)

	for i := 0; i < models.MaxReferralsPerDoctor; i++ {
		require.NoError(t, db.Create(&models.Referral{
			ReferrerDoctorID: referrer.ID,
			ReferredDoctorID: uint(90000 + i), // IDs ficticios, solo para llenar el tope
			Status:           "pending",
		}).Error)
	}

	eleventh := testutil.CreateTestDoctor(t, db, "doc_ref_eleventh", "password123")
	applied := referral.ApplyCodeIfValid(db, eleventh.ID, "CAPCODE1")
	assert.False(t, applied, "el onceavo intento contra el mismo referidor debe rechazarse")
}

// TestGrantRewardIfPending_BothTrialing confirma que, si referidor y
// referido siguen en periodo de prueba, la recompensa se acumula sobre
// TrialEndsAt (nunca se resetea a "ahora + 7").
func TestGrantRewardIfPending_BothTrialing(t *testing.T) {
	db := testutil.SetupTestDB(t)
	referrer := testutil.CreateTestDoctor(t, db, "doc_ref_trial_referrer", "password123")
	referred := testutil.CreateTestDoctor(t, db, "doc_ref_trial_referred", "password123")

	referrerOldEnd := *referrer.TrialEndsAt
	referredOldEnd := *referred.TrialEndsAt

	require.NoError(t, db.Create(&models.Referral{
		ReferrerDoctorID: referrer.ID,
		ReferredDoctorID: referred.ID,
		Status:           "pending",
	}).Error)

	mock := &mockBillingClient{}
	referral.GrantRewardIfPending(context.Background(), db, mock, referred.ID)

	var updatedReferrer, updatedReferred models.Doctor
	require.NoError(t, db.First(&updatedReferrer, referrer.ID).Error)
	require.NoError(t, db.First(&updatedReferred, referred.ID).Error)

	assert.WithinDuration(t, referrerOldEnd.Add(7*24*time.Hour), *updatedReferrer.TrialEndsAt, time.Second)
	assert.WithinDuration(t, referredOldEnd.Add(7*24*time.Hour), *updatedReferred.TrialEndsAt, time.Second)

	var ref models.Referral
	require.NoError(t, db.Where("referred_doctor_id = ?", referred.ID).First(&ref).Error)
	assert.Equal(t, "rewarded", ref.Status)
	assert.NotNil(t, ref.RewardedAt)
}

// TestGrantRewardIfPending_ActiveReferrer_ExtendsStripeSubscription
// confirma que a un referidor que YA paga se le empuja la fecha de
// cobro en Stripe (no hay campo local de vencimiento que tocar para un
// doctor "active" — ver billing.Client.ExtendSubscriptionByDays).
func TestGrantRewardIfPending_ActiveReferrer_ExtendsStripeSubscription(t *testing.T) {
	db := testutil.SetupTestDB(t)
	referrer := testutil.CreateTestDoctor(t, db, "doc_ref_active_referrer", "password123")
	require.NoError(t, db.Model(&referrer).Updates(map[string]any{
		"subscription_status":    "active",
		"stripe_subscription_id": "sub_test_referrer",
	}).Error)
	referred := testutil.CreateTestDoctor(t, db, "doc_ref_active_referred", "password123")

	require.NoError(t, db.Create(&models.Referral{
		ReferrerDoctorID: referrer.ID,
		ReferredDoctorID: referred.ID,
		Status:           "pending",
	}).Error)

	mock := &mockBillingClient{}
	referral.GrantRewardIfPending(context.Background(), db, mock, referred.ID)

	require.Len(t, mock.extendCalls, 1)
	assert.Equal(t, "sub_test_referrer", mock.extendCalls[0].SubscriptionID)
	assert.Equal(t, 7, mock.extendCalls[0].Days)
}

// TestGrantRewardIfPending_Idempotent confirma que una segunda llamada
// (simulando reentrega del webhook de Stripe) no duplica la recompensa.
func TestGrantRewardIfPending_Idempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	referrer := testutil.CreateTestDoctor(t, db, "doc_ref_idem_referrer", "password123")
	referred := testutil.CreateTestDoctor(t, db, "doc_ref_idem_referred", "password123")

	require.NoError(t, db.Create(&models.Referral{
		ReferrerDoctorID: referrer.ID,
		ReferredDoctorID: referred.ID,
		Status:           "pending",
	}).Error)

	mock := &mockBillingClient{}
	referral.GrantRewardIfPending(context.Background(), db, mock, referred.ID)

	var afterFirst models.Doctor
	require.NoError(t, db.First(&afterFirst, referrer.ID).Error)
	firstEnd := *afterFirst.TrialEndsAt

	// Segunda entrega del mismo evento de Stripe.
	referral.GrantRewardIfPending(context.Background(), db, mock, referred.ID)

	var afterSecond models.Doctor
	require.NoError(t, db.First(&afterSecond, referrer.ID).Error)
	assert.WithinDuration(t, firstEnd, *afterSecond.TrialEndsAt, time.Second, "la segunda llamada no debe volver a sumar 7 días")
}

// TestGrantRewardIfPending_NoPendingReferral confirma que llamar la
// función para un doctor que no fue referido (o cuya referencia ya se
// premió) simplemente no hace nada, sin error ni panic.
func TestGrantRewardIfPending_NoPendingReferral(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_ref_no_pending", "password123")

	mock := &mockBillingClient{}
	assert.NotPanics(t, func() {
		referral.GrantRewardIfPending(context.Background(), db, mock, doc.ID)
	})
	assert.Empty(t, mock.extendCalls)
}
