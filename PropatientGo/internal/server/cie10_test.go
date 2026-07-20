package server_test

import (
	"fmt"
	"net/http"
	"testing"

	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCie10_CatalogIsSeeded confirma que SetupTestDB dejó el catálogo
// CIE-10 cargado (ver database.SeedCie10Catalog, llamado ahí una sola
// vez) — sin esto, todos los tests de búsqueda de abajo fallarían por una
// razón ajena a lo que en verdad prueban.
func TestCie10_CatalogIsSeeded(t *testing.T) {
	db := testutil.SetupTestDB(t)
	var count int64
	require.NoError(t, db.Model(&models.Cie10Code{}).Count(&count).Error)
	assert.Greater(t, count, int64(10000), "el catálogo CIE-10 debe tener miles de códigos vigentes cargados")
}

// TestCie10_SearchByCodePrefix confirma la búsqueda por prefijo de código.
func TestCie10_SearchByCodePrefix(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_cie10_code", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/utils/cie10?q=J44", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var results []map[string]any
	decodeJSONList(t, w, &results)
	require.NotEmpty(t, results)
	for _, r := range results {
		code := r["code"].(string)
		assert.Truef(t, len(code) >= 3 && code[:3] == "J44", "código %q no empieza con J44", code)
	}
}

// TestCie10_SearchByName confirma la búsqueda por texto dentro del nombre.
func TestCie10_SearchByName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_cie10_name", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/utils/cie10?q=DIABETES", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var results []map[string]any
	decodeJSONList(t, w, &results)
	require.NotEmpty(t, results)
	for _, r := range results {
		assert.Contains(t, r["name"].(string), "DIABETES")
	}
}

// TestCie10_SearchRequiresMinLength confirma que una consulta demasiado
// corta regresa una lista vacía en vez de medio catálogo.
func TestCie10_SearchRequiresMinLength(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_cie10_short", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodGet, "/api/utils/cie10?q=a", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var results []map[string]any
	decodeJSONList(t, w, &results)
	assert.Empty(t, results)
}

// TestCie10_DiagnosisCode_PersistsAndPreservesHistory confirma que el
// código estructurado se guarda junto al texto libre, y que también
// queda cubierto por el historial inmutable de notas clínicas (ver
// TestNoteHistory_* en audit_test.go, mismo mecanismo).
func TestCie10_DiagnosisCode_PersistsAndPreservesHistory(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_cie10_diag", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "CIE10", "email": "cie10_patient1@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	apptID := createTestAppointment(t, router, token, patientID)

	w = doRequest(t, router, http.MethodPut, fmt.Sprintf("/api/appointments/%d", apptID), token, map[string]any{
		"diagnosis":     "Diabetes mellitus tipo 1, con cetoacidosis",
		"diagnosisCode": "E101",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "E101", decodeJSON(t, w)["diagnosisCode"])

	w = doRequest(t, router, http.MethodPut, fmt.Sprintf("/api/appointments/%d", apptID), token, map[string]any{
		"diagnosis":     "Diabetes mellitus tipo 1, sin mención de complicación",
		"diagnosisCode": "E109",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "E109", decodeJSON(t, w)["diagnosisCode"])

	w = doRequest(t, router, http.MethodGet, fmt.Sprintf("/api/appointments/%d/note-history", apptID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var history []map[string]any
	decodeJSONList(t, w, &history)
	require.Len(t, history, 2)
	assert.Equal(t, "E101", history[0]["previousDiagnosisCode"], "la versión más reciente debe preservar el código anterior, E101")
	assert.Equal(t, "", history[1]["previousDiagnosisCode"], "la primera edición preserva el estado original: sin código")
}
