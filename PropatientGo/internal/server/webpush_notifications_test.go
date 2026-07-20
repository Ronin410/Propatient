package server_test

import (
	"context"
	"net/http"
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

// TestPublicAppointment_NotifiesDoctorByPush confirma que, con VAPID
// configurado y el doctor suscrito, una solicitud de cita pública dispara
// una notificación push al dispositivo del doctor — el equivalente al
// TestPublicAppointment_NotifiesPatientAndDoctorByWhatsApp pero por push.
func TestPublicAppointment_NotifiesDoctorByPush(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	push := newMockWebpushClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, push)

	doc := testutil.CreateTestDoctor(t, db, "doc_push_booking", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"public_listed": true, "public_slug": "dr-push-booking-1",
	}).Error)
	require.NoError(t, db.Create(&models.PushSubscription{
		DoctorID: doc.ID, Endpoint: "https://push.example/doc-device-1", P256dhKey: "p", AuthKey: "a",
	}).Error)

	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Sofía",
		"patientLastName":     "Nuñez",
		"patientPhone":        "5551237890",
		"patientEmail":        "sofia.push@test.local",
		"dataConsent":         true,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// El envío va en segundo plano (no bloquea la respuesta HTTP).
	push.waitForCallsTo(t, "https://push.example/doc-device-1", 1)
}

// TestPublicAppointment_NoPushSubscriptionDoesNotFail confirma que un
// doctor sin ninguna suscripción push no rompe el flujo de agendado — el
// mismo criterio de "mejor esfuerzo" que el resto de los avisos.
func TestPublicAppointment_NoPushSubscriptionDoesNotFail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	push := newMockWebpushClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, push)

	doc := testutil.CreateTestDoctor(t, db, "doc_push_nosub", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{
		"public_listed": true, "public_slug": "dr-push-nosub-1",
	}).Error)

	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Carlos",
		"patientLastName":     "Ruiz",
		"patientPhone":        "5559990000",
		"patientEmail":        "carlos.push@test.local",
		"dataConsent":         true,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, 0, push.callCount())
}

// TestSavePushSubscription_CreatesAndUpsertsByEndpoint cubre el endpoint
// que usa el frontend para guardar la suscripción de un dispositivo, y
// confirma que volver a mandar el mismo Endpoint actualiza en vez de
// duplicar (ver handlers.SavePushSubscription).
func TestSavePushSubscription_CreatesAndUpsertsByEndpoint(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_push_save", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	body := map[string]any{
		"endpoint":  "https://push.example/save-1",
		"p256dhKey": "p-original",
		"authKey":   "a-original",
	}
	w := doRequest(t, router, http.MethodPost, "/api/doctor/push-subscriptions", token, body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var subs []models.PushSubscription
	require.NoError(t, db.Where("doctor_id = ?", doc.ID).Find(&subs).Error)
	require.Len(t, subs, 1)
	assert.Equal(t, "p-original", subs[0].P256dhKey)

	// Reenviar el mismo Endpoint con llaves distintas debe actualizar la
	// fila existente, no crear una segunda.
	body["p256dhKey"] = "p-updated"
	w = doRequest(t, router, http.MethodPost, "/api/doctor/push-subscriptions", token, body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	require.NoError(t, db.Where("doctor_id = ?", doc.ID).Find(&subs).Error)
	require.Len(t, subs, 1, "no debe duplicar la suscripción del mismo endpoint")
	assert.Equal(t, "p-updated", subs[0].P256dhKey)
}

// TestDeletePushSubscription_RemovesOnlyOwnSubscription confirma que un
// doctor puede quitar la suscripción de su propio dispositivo, y que no
// puede borrar la de otro doctor adivinando el endpoint (scoping por
// doctorID, ver handlers.DeletePushSubscription).
func TestDeletePushSubscription_RemovesOnlyOwnSubscription(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil, nil)

	docA := testutil.CreateTestDoctor(t, db, "doc_push_del_a", "password123")
	docB := testutil.CreateTestDoctor(t, db, "doc_push_del_b", "password123")
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	require.NoError(t, db.Create(&models.PushSubscription{
		DoctorID: docA.ID, Endpoint: "https://push.example/del-a", P256dhKey: "p", AuthKey: "a",
	}).Error)

	// docB intenta borrar la suscripción de docA — no debe pasar nada.
	w := doRequest(t, router, http.MethodDelete, "/api/doctor/push-subscriptions", tokenB, map[string]any{
		"endpoint": "https://push.example/del-a",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("doctor_id = ?", docA.ID).Count(&count).Error)
	assert.Equal(t, int64(1), count, "docB no debe poder borrar la suscripción de docA")

	// docA sí puede borrar la suya.
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	w = doRequest(t, router, http.MethodDelete, "/api/doctor/push-subscriptions", tokenA, map[string]any{
		"endpoint": "https://push.example/del-a",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	require.NoError(t, db.Model(&models.PushSubscription{}).Where("doctor_id = ?", docA.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}
