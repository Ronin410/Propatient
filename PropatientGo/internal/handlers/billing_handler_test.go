package handlers

import (
	"testing"
	"time"

	"propatient-api/internal/models"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubscriptionStatusUpdates_SetsPastDueSinceOnlyOnce cubre el punto
// central de subscriptionStatusUpdates: PastDueSince debe marcarse la
// PRIMERA vez que el webhook reporta "past_due" (para arrancar el periodo
// de gracia, ver billing.PastDuePaymentGraceDuration), pero NO reiniciarse
// con cada reintento de cobro que Stripe hace mientras la suscripción
// sigue en ese mismo estado — de lo contrario el dueño nunca perdería
// acceso mientras Stripe siga reintentando indefinidamente. Al recuperarse
// (o cancelarse), debe limpiarse.
func TestSubscriptionStatusUpdates_SetsPastDueSinceOnlyOnce(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_pastdue_updates", "password123")

	require.NoError(t, db.Model(&models.Doctor{}).Where("id = ?", doc.ID).Updates(subscriptionStatusUpdates("past_due")).Error)
	var afterFirstFailure models.Doctor
	require.NoError(t, db.First(&afterFirstFailure, doc.ID).Error)
	require.NotNil(t, afterFirstFailure.PastDueSince)
	firstMark := *afterFirstFailure.PastDueSince

	time.Sleep(10 * time.Millisecond)

	// Reintento de Stripe mientras SIGUE en past_due: no debe mover la marca.
	require.NoError(t, db.Model(&models.Doctor{}).Where("id = ?", doc.ID).Updates(subscriptionStatusUpdates("past_due")).Error)
	var afterRetry models.Doctor
	require.NoError(t, db.First(&afterRetry, doc.ID).Error)
	require.NotNil(t, afterRetry.PastDueSince)
	assert.WithinDuration(t, firstMark, *afterRetry.PastDueSince, time.Millisecond, "un reintento no debe reiniciar el periodo de gracia")

	// El pago se recupera: limpia past_due_since.
	require.NoError(t, db.Model(&models.Doctor{}).Where("id = ?", doc.ID).Updates(subscriptionStatusUpdates("active")).Error)
	var afterRecovery models.Doctor
	require.NoError(t, db.First(&afterRecovery, doc.ID).Error)
	assert.Nil(t, afterRecovery.PastDueSince)
}
