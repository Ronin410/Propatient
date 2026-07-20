package server_test

import (
	"net/http"
	"testing"

	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAcceptTerms_RecordsEvidence cubre el Caso 1 del análisis legal: un
// doctor autenticado que acepta los Términos y Condiciones + Aviso de
// Privacidad debe quedar con evidencia (fecha, versión, IP) en su propio
// registro, y accept-terms debe poder llamarse aunque el doctor todavía no
// haya completado el resto del onboarding.
func TestAcceptTerms_RecordsEvidence(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_accept_terms", "password123")
	require.Nil(t, doc.TermsAcceptedAt, "un doctor recién creado no debe tener términos aceptados")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/user/accept-terms", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Equal(t, models.CurrentLegalNoticeVersion, body["termsAcceptedVersion"])

	var updated models.Doctor
	require.NoError(t, db.First(&updated, doc.ID).Error)
	require.NotNil(t, updated.TermsAcceptedAt)
	assert.Equal(t, models.CurrentLegalNoticeVersion, updated.TermsAcceptedVersion)
	assert.NotEmpty(t, updated.TermsAcceptedIP)
}

// TestAcceptTerms_RequiresAuth confirma que un doctor sin sesión no puede
// registrar una aceptación en nombre de nadie.
func TestAcceptTerms_RequiresAuth(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodPost, "/api/user/accept-terms", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestAcceptTerms_ExposedInGetCurrentDoctor confirma que el doctor
// autenticado puede ver si ya aceptó los términos vía /doctor/me (el mismo
// campo que expone auth.GoogleLoginHandler como "terminosAceptados" en el
// login, usado por el frontend como primer gate del onboarding — ver
// OnboardingGuard.tsx).
func TestAcceptTerms_ExposedInGetCurrentDoctor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_terms_status", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/me", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeJSON(t, w)
	assert.Nil(t, body["termsAcceptedAt"], "sin aceptar, termsAcceptedAt debe venir vacío en /doctor/me")

	w = doRequest(t, router, http.MethodPost, "/api/user/accept-terms", token, nil)
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/doctor/me", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body = decodeJSON(t, w)
	assert.NotNil(t, body["termsAcceptedAt"], "después de aceptar, /doctor/me debe reflejarlo")
}
