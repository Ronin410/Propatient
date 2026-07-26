package server_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"propatient-api/internal/billing"
	"propatient-api/internal/geocoding"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extractUploadToken saca el token del final de una uploadUrl tipo
// "http://localhost:5173/public-upload/abc123...".
func extractUploadToken(t *testing.T, uploadURL string) string {
	t.Helper()
	parts := strings.Split(uploadURL, "/public-upload/")
	require.Len(t, parts, 2, "uploadUrl con formato inesperado: %s", uploadURL)
	return parts[1]
}

// TestPublicUpload_FullFlow cubre el flujo completo del QR de "sube tus
// documentos antes de la cita": el doctor pide el link, el paciente (sin
// sesión) lo consulta y sube un archivo, y el documento aparece en el
// expediente de la cita.
func TestPublicUpload_FullFlow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_public_upload", "password123")
	doc.FullName = "Dra. Upload"
	require.NoError(t, db.Save(&doc).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Upload", "email": "upload@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	// El doctor pide el link/QR.
	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(apptID)+"/upload-link", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	uploadURL, _ := decodeJSON(t, w)["uploadUrl"].(string)
	require.NotEmpty(t, uploadURL)
	uploadToken := extractUploadToken(t, uploadURL)

	// Pedir el link una segunda vez debe devolver el MISMO token (no se
	// invalida el QR ya generado/impreso/enviado cada vez que se abre la
	// pantalla).
	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(apptID)+"/upload-link", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	secondURL, _ := decodeJSON(t, w)["uploadUrl"].(string)
	assert.Equal(t, uploadURL, secondURL)

	// El paciente (sin sesión) consulta la info del link.
	w = doRequest(t, router, http.MethodGet, "/api/public/upload/"+uploadToken, "", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	info := decodeJSON(t, w)
	assert.Equal(t, "Paciente", info["patientName"])
	assert.Equal(t, "Dra. Upload", info["doctorName"])
	assert.Equal(t, float64(0), info["documentCount"])

	// El paciente sube un archivo (sin sesión, multipart).
	w = doMultipartRequest(t, router, http.MethodPost, "/api/public/upload/"+uploadToken, "",
		nil, "files", "estudio.pdf", minimalPDFBytes)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	uploadResp := decodeJSON(t, w)
	assert.Equal(t, float64(1), uploadResp["saved"])
	assert.Equal(t, 1, mockStorage.savedKeyCount())

	// El documento aparece en el expediente de la cita (vista del doctor).
	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(apptID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	documents, ok := body["documents"].([]any)
	require.True(t, ok)
	require.Len(t, documents, 1)

	// documentCount ahora refleja el archivo subido.
	w = doRequest(t, router, http.MethodGet, "/api/public/upload/"+uploadToken, "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(1), decodeJSON(t, w)["documentCount"])
}

// TestPublicUpload_RejectsInvalidToken cubre el token inventado tanto para
// consultar la info como para subir un archivo.
func TestPublicUpload_RejectsInvalidToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil, geocoding.NewClient(), nil, nil)

	w := doRequest(t, router, http.MethodGet, "/api/public/upload/token-inventado", "", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = doMultipartRequest(t, router, http.MethodPost, "/api/public/upload/token-inventado", "",
		nil, "files", "estudio.pdf", minimalPDFBytes)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, 0, mockStorage.savedKeyCount())
}

// TestPublicUpload_StaffCannotGenerateLink confirma que el link solo lo
// puede pedir el doctor dueño del consultorio, no el personal — mismo
// criterio que el resto de la gestión de documentos de la cita.
func TestPublicUpload_StaffCannotGenerateLink(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_public_upload_staff", "password123")
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)
	staff := testutil.CreateTestStaff(t, db, doc.ID, "staff_upload@consultorio.test", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodPost, "/api/patients", docToken, map[string]any{
		"firstName": "Paciente", "lastName": "StaffUpload", "email": "staffupload@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", docToken, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(apptID)+"/upload-link", staffToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
