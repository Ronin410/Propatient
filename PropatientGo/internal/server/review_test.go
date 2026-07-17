package server_test

import (
	"context"
	"net/http"
	"strconv"
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

// TestReview_FullLifecycle cubre el flujo completo: marcar una cita como
// COMPLETED manda un WhatsApp con el link de reseña, el paciente la envía,
// el doctor la ve pendiente de aprobar, la aprueba, y ahí sí aparece en el
// perfil público con el promedio correcto.
func TestReview_FullLifecycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	mockWA := newMockWhatsAppClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, geocoding.NewClient(), mockWA)

	doc := testutil.CreateTestDoctor(t, db, "doc_review_full", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-review-full"}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z", "service": "Consulta",
		"patientFirstName": "Marcela", "patientLastName": "Reseñas", "patientPhone": "5512340000",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	appointmentID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(appointmentID), token, map[string]any{
		"status": "COMPLETED",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	mockWA.waitForCallsTo(t, "5512340000", 1)

	var review models.Review
	require.NoError(t, db.Where("appointment_id = ?", appointmentID).First(&review).Error)
	require.NotEmpty(t, review.Token)

	// Guardar de nuevo la misma cita (ej. otro cambio de notas) no debe
	// mandar una segunda invitación.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(appointmentID), token, map[string]any{
		"status": "COMPLETED", "notes": "nota extra",
	})
	require.Equal(t, http.StatusOK, w.Code)
	var reviewCount int64
	db.Model(&models.Review{}).Where("appointment_id = ?", appointmentID).Count(&reviewCount)
	assert.Equal(t, int64(1), reviewCount)

	// El paciente abre el link y envía su reseña.
	w = doRequest(t, router, http.MethodGet, "/api/public/reviews/"+review.Token, "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	invite := decodeJSON(t, w)
	assert.Equal(t, "Marcela", invite["patientName"])
	assert.Equal(t, false, invite["alreadyRated"])

	w = doRequest(t, router, http.MethodPost, "/api/public/reviews/"+review.Token, "", map[string]any{
		"rating": 5, "comment": "Excelente consulta.",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// No se puede reenviar con el mismo token.
	w = doRequest(t, router, http.MethodPost, "/api/public/reviews/"+review.Token, "", map[string]any{
		"rating": 1, "comment": "otro intento",
	})
	assert.Equal(t, http.StatusConflict, w.Code)

	// Todavía no debe verse en el perfil público (falta aprobación).
	w = doRequest(t, router, http.MethodGet, "/api/public/doctors/dr-review-full", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	publicProfile := decodeJSON(t, w)
	assert.Nil(t, publicProfile["reviews"])

	// El doctor la ve en su lista de reseñas, sin aprobar.
	w = doRequest(t, router, http.MethodGet, "/api/reviews", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var reviews []map[string]any
	decodeJSONList(t, w, &reviews)
	require.Len(t, reviews, 1)
	assert.Equal(t, false, reviews[0]["approved"])
	assert.Equal(t, "Marcela", reviews[0]["patientFirstName"])
	reviewID := int(reviews[0]["id"].(float64))

	// La aprueba.
	w = doRequest(t, router, http.MethodPut, "/api/reviews/"+strconv.Itoa(reviewID)+"/approve", token, map[string]any{
		"approved": true,
	})
	require.Equal(t, http.StatusOK, w.Code)

	// Ahora sí aparece en el perfil público.
	w = doRequest(t, router, http.MethodGet, "/api/public/doctors/dr-review-full", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	publicProfile = decodeJSON(t, w)
	publicReviews, ok := publicProfile["reviews"].([]any)
	require.True(t, ok)
	require.Len(t, publicReviews, 1)
	assert.Equal(t, float64(5), publicProfile["reviewsAverage"])
}

// TestReview_ExpiredOrInvalidTokenRejected confirma que un token vencido o
// inventado no deja enviar una reseña.
func TestReview_ExpiredOrInvalidTokenRejected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_review_expired", "password123")
	patient := models.Patient{FirstName: "Paciente", LastName: "Vencido"}
	require.NoError(t, db.Create(&patient).Error)
	appointment := models.Appointment{DoctorID: doc.ID, PatientID: patient.ID, AppointmentDateTime: time.Now(), Status: "COMPLETED"}
	require.NoError(t, db.Create(&appointment).Error)

	expired := time.Now().UTC().Add(-time.Hour)
	review := models.Review{DoctorID: doc.ID, PatientID: patient.ID, AppointmentID: appointment.ID, Token: "token-vencido", TokenExpiresAt: &expired}
	require.NoError(t, db.Create(&review).Error)

	w := doRequest(t, router, http.MethodGet, "/api/public/reviews/token-vencido", "", nil)
	assert.Equal(t, http.StatusGone, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/public/reviews/token-vencido", "", map[string]any{"rating": 5})
	assert.Equal(t, http.StatusGone, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/public/reviews/token-que-nunca-existio", "", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestReview_RatingOutOfRangeRejected confirma que el backend valida el
// rango de la calificación (1-5), no solo el frontend.
func TestReview_RatingOutOfRangeRejected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_review_range", "password123")
	patient := models.Patient{FirstName: "Paciente", LastName: "Rango"}
	require.NoError(t, db.Create(&patient).Error)
	appointment := models.Appointment{DoctorID: doc.ID, PatientID: patient.ID, AppointmentDateTime: time.Now(), Status: "COMPLETED"}
	require.NoError(t, db.Create(&appointment).Error)

	future := time.Now().UTC().Add(time.Hour)
	review := models.Review{DoctorID: doc.ID, PatientID: patient.ID, AppointmentID: appointment.ID, Token: "token-rango", TokenExpiresAt: &future}
	require.NoError(t, db.Create(&review).Error)

	w := doRequest(t, router, http.MethodPost, "/api/public/reviews/token-rango", "", map[string]any{"rating": 0})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/public/reviews/token-rango", "", map[string]any{"rating": 6})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestReview_OnlyDoctorCanListOrApprove confirma que el personal no puede
// ver ni aprobar reseñas — es una herramienta de reputación/marketing del
// doctor, no algo operativo del día a día.
func TestReview_OnlyDoctorCanListOrApprove(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_review_staffblock", "password123")
	staff := testutil.CreateTestStaff(t, db, doc.ID, "personal_review@test.local", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodGet, "/api/reviews", staffToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = doRequest(t, router, http.MethodPut, "/api/reviews/1/approve", staffToken, map[string]any{"approved": true})
	assert.Equal(t, http.StatusForbidden, w.Code)
}
