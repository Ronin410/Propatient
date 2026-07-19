package server_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"propatient-api/internal/billing"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/storage"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateCurrentDoctor_PublicListing_GeneratesSlugAndGeocodes cubre el
// flujo de opt-in: el doctor activa el listado público con una dirección,
// y el perfil queda con slug propio y coordenadas geocodificadas.
func TestUpdateCurrentDoctor_PublicListing_GeneratesSlugAndGeocodes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	geo := newMockGeocodingClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, geo, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_public_listing", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doMultipartRequest(t, router, http.MethodPut, "/api/doctor/me", token, map[string]string{
		"fullName":     "José Ángel Pérez",
		"publicListed": "true",
		"publicBio":    "Urólogo con 10 años de experiencia.",
		"address":      "Av. Insurgentes Sur 123, CDMX",
	}, "", "", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var updated models.Doctor
	require.NoError(t, db.First(&updated, doc.ID).Error)
	assert.True(t, updated.PublicListed)
	assert.NotEmpty(t, updated.PublicSlug)
	assert.Contains(t, updated.PublicSlug, "jose-angel-perez")
	require.NotNil(t, updated.Latitude)
	require.NotNil(t, updated.Longitude)
	assert.Equal(t, 25.789, *updated.Latitude)
	assert.Equal(t, 1, geo.callCount())

	// Guardar de nuevo sin cambiar la dirección no debe volver a geocodificar.
	w = doMultipartRequest(t, router, http.MethodPut, "/api/doctor/me", token, map[string]string{
		"fullName":     "José Ángel Pérez",
		"publicListed": "true",
		"publicBio":    "Bio actualizada.",
		"address":      "Av. Insurgentes Sur 123, CDMX",
	}, "", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, geo.callCount(), "no debe volver a geocodificar si la dirección no cambió")

	// El slug no debe cambiar tampoco entre guardados (rompería enlaces ya compartidos).
	var updatedAgain models.Doctor
	require.NoError(t, db.First(&updatedAgain, doc.ID).Error)
	assert.Equal(t, updated.PublicSlug, updatedAgain.PublicSlug)
}

// TestPublicDoctors_OnlyShowsOptedInAndActive confirma las dos reglas del
// directorio: solo aparecen doctores que activaron el listado, y solo si
// tienen prueba vigente o suscripción activa.
func TestPublicDoctors_OnlyShowsOptedInAndActive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil)

	listedActive := testutil.CreateTestDoctor(t, db, "doc_listed_active", "password123")
	require.NoError(t, db.Model(&listedActive).Updates(map[string]any{
		"public_listed": true, "public_slug": "dr-activo-1", "full_name": "Dr. Activo",
	}).Error)

	notListed := testutil.CreateTestDoctor(t, db, "doc_not_listed", "password123")
	_ = notListed

	listedButExpired := testutil.CreateTestDoctor(t, db, "doc_listed_expired", "password123")
	expired := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&listedButExpired).Updates(map[string]any{
		"public_listed": true, "public_slug": "dr-vencido-1", "trial_ends_at": expired,
	}).Error)

	w := doRequest(t, router, http.MethodGet, "/api/public/doctors", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var doctors []map[string]any
	decodeJSONList(t, w, &doctors)

	require.Len(t, doctors, 1)
	assert.Equal(t, "Dr. Activo", doctors[0]["fullName"])
}

// TestPublicAppointment_FullLifecycle cubre el flujo completo: alguien sin
// cuenta agenda con un doctor público, la cita nace pendiente de
// confirmación (no ocupa el horario todavía), y el doctor la confirma.
func TestPublicAppointment_FullLifecycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	mockCal := newMockCalendarClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, mockCal, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_public_booking", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-booking-1"}).Error)
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"reason":              "Dolor de espalda",
		"patientFirstName":    "Carlos",
		"patientLastName":     "Ramírez",
		"patientPhone":        "5551234567",
		"patientEmail":        "carlos.publico@test.local",
		"dataConsent":         true,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// La cita nace PENDING_CONFIRMATION: no debe aparecer en la agenda normal del doctor.
	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=PENDING", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var pending []map[string]any
	decodeJSONList(t, w, &pending)
	assert.Empty(t, pending, "una solicitud pública sin confirmar no debe aparecer como cita PENDING normal")

	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=PENDING_CONFIRMATION", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var requests []map[string]any
	decodeJSONList(t, w, &requests)
	require.Len(t, requests, 1)
	apptID := int(requests[0]["id"].(float64))
	assert.Equal(t, 0, mockCal.createdCount(), "no debe sincronizarse a Calendar hasta que el doctor confirme")

	// El doctor confirma.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID)+"/confirm", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=PENDING", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	decodeJSONList(t, w, &pending)
	require.Len(t, pending, 1, "tras confirmar, la cita ya debe verse como PENDING normal")

	// Confirmar otra vez debe fallar (ya no está en PENDING_CONFIRMATION).
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID)+"/confirm", docToken, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestPublicAppointment_RejectsDoctorNotListed evita que alguien agende
// adivinando el ID de un doctor que nunca activó el directorio público.
func TestPublicAppointment_RejectsDoctorNotListed(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_not_public", "password123")

	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Ana",
		"patientLastName":     "López",
		"patientPhone":        "5559876543",
		"patientEmail":        "ana.publico@test.local",
		"dataConsent":         true,
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestPublicAppointment_HiddenFromCalendarAndTodaySummaryBeforeConfirmation
// cubre el bug reportado: una solicitud pública sin confirmar no debe verse
// en la agenda general (sin filtro de status) ni en el resumen "hoy" del
// dashboard, y no debe poder abrirse ni modificarse como si fuera una cita
// real hasta que el consultorio la confirme.
func TestPublicAppointment_HiddenFromCalendarAndTodaySummaryBeforeConfirmation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_hidden_pending", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-hidden-1"}).Error)
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	todayNoon := time.Now().UTC().Truncate(24 * time.Hour).Add(12 * time.Hour)
	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": todayNoon.Format(time.RFC3339),
		"reason":              "Chequeo",
		"patientFirstName":    "Mario",
		"patientLastName":     "Solís",
		"patientPhone":        "5550001111",
		"patientEmail":        "mario.hidden@test.local",
		"dataConsent":         true,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// No debe verse en la agenda general (GetAppointments sin status).
	w = doRequest(t, router, http.MethodGet, "/api/appointments", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var all []map[string]any
	decodeJSONList(t, w, &all)
	assert.Empty(t, all, "una solicitud pública sin confirmar no debe verse en la agenda general")

	// No debe contarse ni listarse en el resumen de "hoy", aunque su fecha
	// sea hoy — así "Iniciar Atención" nunca aparece antes de aceptarla.
	w = doRequest(t, router, http.MethodGet, "/api/dashboard/summary", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	summary := decodeJSON(t, w)
	assert.Equal(t, float64(0), summary["todayCount"])
	todayAppointments, ok := summary["todayAppointments"].([]any)
	require.True(t, ok)
	assert.Empty(t, todayAppointments)

	// Tampoco se puede abrir como el detalle de una consulta...
	w = doRequest(t, router, http.MethodGet, "/api/appointments", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=PENDING_CONFIRMATION", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var requests []map[string]any
	decodeJSONList(t, w, &requests)
	require.Len(t, requests, 1)
	apptID := strconv.Itoa(int(requests[0]["id"].(float64)))

	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+apptID, docToken, nil)
	assert.Equal(t, http.StatusConflict, w.Code)

	// ...ni modificarse (reprogramar, guardar notas de consulta, etc.).
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+apptID, docToken, map[string]any{
		"appointmentDateTime": todayNoon.Add(time.Hour).Format(time.RFC3339),
	})
	assert.Equal(t, http.StatusConflict, w.Code)
}

// TestPublicAppointment_DedupesPatientByPhoneWhenAlreadyDoctorsPatient cubre
// el otro caso pedido: si la persona que agenda ya es paciente de ESTE
// doctor pero escribe un correo distinto al que tenía registrado, debe
// reconocerse por teléfono y no crear un paciente duplicado.
func TestPublicAppointment_DedupesPatientByPhoneWhenAlreadyDoctorsPatient(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_phone_dedupe", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-phone-dedupe-1"}).Error)
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", docToken, map[string]any{
		"firstName": "Lucía", "lastName": "Ramírez", "phone": "5557778888", "email": "lucia.original@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	existingPatientID := int(decodeJSON(t, w)["id"].(float64))

	// Agenda en línea con OTRO correo pero el MISMO teléfono ya registrado
	// para este doctor.
	w = doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-10T09:00:00Z",
		"patientFirstName":    "Lucía",
		"patientLastName":     "Ramírez",
		"patientPhone":        "5557778888",
		"patientEmail":        "lucia.nuevo-correo@test.local",
		"dataConsent":         true,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var count int64
	db.Model(&models.Patient{}).Where("phone = ?", "5557778888").Count(&count)
	assert.Equal(t, int64(1), count, "no debe crear un paciente duplicado si el teléfono ya coincide con uno del doctor")

	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=PENDING_CONFIRMATION", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var requests []map[string]any
	decodeJSONList(t, w, &requests)
	require.Len(t, requests, 1)
	assert.Equal(t, float64(existingPatientID), requests[0]["patientId"], "la solicitud debe quedar ligada al paciente ya existente, no a uno nuevo")
}

// TestPublicAppointment_DedupesPatientByEmail confirma que dos solicitudes
// con el mismo correo reutilizan el mismo paciente, sin duplicarlo.
func TestPublicAppointment_DedupesPatientByEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_public_dedupe", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-dedupe-1"}).Error)

	body := map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Luis",
		"patientLastName":     "Torres",
		"patientPhone":        "5551112222",
		"patientEmail":        "luis.dedupe@test.local",
		"dataConsent":         true,
	}

	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", body)
	require.Equal(t, http.StatusCreated, w.Code)

	body["appointmentDateTime"] = "2026-09-05T12:00:00Z"
	w = doRequest(t, router, http.MethodPost, "/api/public/appointments", "", body)
	require.Equal(t, http.StatusCreated, w.Code)

	var count int64
	db.Model(&models.Patient{}).Where("email = ?", "luis.dedupe@test.local").Count(&count)
	assert.Equal(t, int64(1), count, "no debe crear un paciente duplicado en la segunda solicitud")
}

// TestPublicAppointment_RequiresDataConsent cubre el consentimiento
// obligatorio de tratamiento de datos de salud: sin dataConsent=true, la
// solicitud se rechaza y no debe crear ni cita ni paciente.
func TestPublicAppointment_RequiresDataConsent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_consent_required", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-consent-1"}).Error)

	baseBody := map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Sin",
		"patientLastName":     "Consentimiento",
		"patientPhone":        "5550009999",
		"patientEmail":        "sin.consentimiento@test.local",
	}

	// Campo ausente.
	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", baseBody)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Explícitamente en false.
	baseBody["dataConsent"] = false
	w = doRequest(t, router, http.MethodPost, "/api/public/appointments", "", baseBody)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var count int64
	db.Model(&models.Appointment{}).Count(&count)
	assert.Equal(t, int64(0), count, "no debe crear ninguna cita sin el consentimiento aceptado")

	// Con dataConsent=true sí debe funcionar.
	baseBody["dataConsent"] = true
	w = doRequest(t, router, http.MethodPost, "/api/public/appointments", "", baseBody)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

// TestPublicAppointment_MarksPatientAsMinor cubre el caso de un padre/madre
// o tutor agendando para un paciente menor de edad: el teléfono/correo del
// formulario son los del adulto (el menor no tiene los suyos propios), y
// isMinorPatient debe quedar guardado en el expediente del paciente para
// que el doctor sepa que el consentimiento lo dio su tutor, no el paciente.
func TestPublicAppointment_MarksPatientAsMinor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testStorage, _ := storage.NewClient(context.Background(), storage.Config{})
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, testStorage, billing.Config{}, nil, newMockGeocodingClient(), nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_minor_patient", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-minor-1"}).Error)
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/public/appointments", "", map[string]any{
		"doctorId":            doc.ID,
		"appointmentDateTime": "2026-09-01T10:00:00Z",
		"patientFirstName":    "Sofía",
		"patientLastName":     "Niña",
		"patientPhone":        "5559998888", // teléfono del padre/tutor, no del menor
		"patientEmail":        "papa.de.sofia@test.local",
		"dataConsent":         true,
		"isMinorPatient":      true,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = doRequest(t, router, http.MethodGet, "/api/appointments?status=PENDING_CONFIRMATION", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var requests []map[string]any
	decodeJSONList(t, w, &requests)
	require.Len(t, requests, 1)
	patient, ok := requests[0]["patient"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, patient["isMinor"], "el paciente debe quedar marcado como menor de edad")
}
