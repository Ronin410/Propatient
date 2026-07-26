package webpush_test

import (
	"context"
	"errors"
	"testing"

	"propatient-api/internal/models"
	"propatient-api/internal/testutil"
	"propatient-api/internal/webpush"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingClient simula webpush.Client sin tocar la red: guarda cada
// llamada, y puede configurarse para responder distinto según el
// endpoint (para simular una suscripción expirada vs. un fallo real).
type recordingClient struct {
	calls           []string
	expiredEndpoint string
	failEndpoint    string
}

func (r *recordingClient) SendNotification(ctx context.Context, sub webpush.Subscription, payload []byte) error {
	r.calls = append(r.calls, sub.Endpoint)
	if sub.Endpoint == r.expiredEndpoint {
		return webpush.ErrSubscriptionExpired
	}
	if sub.Endpoint == r.failEndpoint {
		return errors.New("fallo simulado de red")
	}
	return nil
}

// TestSendToDoctor_SendsToEveryDeviceAndPrunesExpired cubre el caso
// central: un doctor con varios dispositivos suscritos recibe la
// notificación en todos, y la suscripción que el proveedor push marca
// como expirada (404/410) se borra sola sin afectar a las demás.
func TestSendToDoctor_SendsToEveryDeviceAndPrunesExpired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_webpush_multi", "password123")

	live := models.PushSubscription{DoctorID: doc.ID, Endpoint: "https://push.example/live", P256dhKey: "p1", AuthKey: "a1"}
	expired := models.PushSubscription{DoctorID: doc.ID, Endpoint: "https://push.example/expired", P256dhKey: "p2", AuthKey: "a2"}
	failing := models.PushSubscription{DoctorID: doc.ID, Endpoint: "https://push.example/failing", P256dhKey: "p3", AuthKey: "a3"}
	require.NoError(t, db.Create(&live).Error)
	require.NoError(t, db.Create(&expired).Error)
	require.NoError(t, db.Create(&failing).Error)

	client := &recordingClient{expiredEndpoint: expired.Endpoint, failEndpoint: failing.Endpoint}
	webpush.SendToDoctor(context.Background(), db, client, doc.ID, []byte(`{"title":"x"}`))

	assert.ElementsMatch(t, []string{live.Endpoint, expired.Endpoint, failing.Endpoint}, client.calls,
		"debe intentar mandar a las tres suscripciones, sin importar si alguna falla")

	var remaining []models.PushSubscription
	require.NoError(t, db.Where("doctor_id = ?", doc.ID).Find(&remaining).Error)
	var remainingEndpoints []string
	for _, s := range remaining {
		remainingEndpoints = append(remainingEndpoints, s.Endpoint)
	}
	assert.ElementsMatch(t, []string{live.Endpoint, failing.Endpoint}, remainingEndpoints,
		"la expirada debe borrarse; la que solo falló por red debe quedarse para reintentar después")
}

// TestSendToDoctor_NoSubscriptionsDoesNothing confirma que un doctor sin
// ninguna suscripción no dispara ninguna llamada (ni error).
func TestSendToDoctor_NoSubscriptionsDoesNothing(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_webpush_none", "password123")

	client := &recordingClient{}
	webpush.SendToDoctor(context.Background(), db, client, doc.ID, []byte(`{"title":"x"}`))

	assert.Empty(t, client.calls)
}

// TestSendToDoctor_NilClientDoesNothing confirma el mismo criterio de
// "mejor esfuerzo" que whatsapp.SendWithFallback: sin cliente configurado
// (VAPID no configurado), no hace nada — ni siquiera consulta la DB.
func TestSendToDoctor_NilClientDoesNothing(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_webpush_nil", "password123")

	// No debe entrar en pánico ni tocar la DB con un client nil.
	webpush.SendToDoctor(context.Background(), db, nil, doc.ID, []byte(`{"title":"x"}`))
}
