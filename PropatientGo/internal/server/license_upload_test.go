package server_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"propatient-api/internal/billing"
	"propatient-api/internal/geocoding"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doLicenseUploadRequest arma un POST multipart con campos de texto y DOS
// archivos (ineDocument + cedulaDocument) — doMultipartRequest (ver
// storage_test.go) solo soporta un archivo, y este endpoint siempre
// requiere ambos.
func doLicenseUploadRequest(t *testing.T, router http.Handler, token string, fields map[string]string, ineBytes, cedulaBytes []byte) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for k, v := range fields {
		require.NoError(t, writer.WriteField(k, v))
	}
	if ineBytes != nil {
		part, err := writer.CreateFormFile("ineDocument", "ine.png")
		require.NoError(t, err)
		_, err = part.Write(ineBytes)
		require.NoError(t, err)
	}
	if cedulaBytes != nil {
		part, err := writer.CreateFormFile("cedulaDocument", "cedula.png")
		require.NoError(t, err)
		_, err = part.Write(cedulaBytes)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/user/update-license-full", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestLicenseUpload_RequiresCedulaDocument confirma la regla NOM-024/LFPDPPP
// de este ciclo: ya no basta con subir la identificación oficial (INE) —
// también hay que adjuntar el documento de la cédula profesional en sí, para
// que el revisor humano en AdminPendingDoctors pueda cotejarla contra el
// número capturado en vez de solo confiar en el texto.
func TestLicenseUpload_RequiresCedulaDocument(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_license_upload", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	fields := map[string]string{"licenseNumber": "12345678"}

	// Sin cedulaDocument: rechazado.
	w := doLicenseUploadRequest(t, router, token, fields, minimalPNGBytes, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// Con ambos documentos: aceptado, y ambas rutas quedan guardadas.
	w = doLicenseUploadRequest(t, router, token, fields, minimalPNGBytes, minimalPNGBytes)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 2, mockStorage.savedKeyCount())

	var reloaded struct {
		CedulaDocumentPath string `gorm:"column:cedula_document_path"`
		IneDocumentPath    string `gorm:"column:ine_document_path"`
	}
	require.NoError(t, db.Table("doctors").Select("cedula_document_path, ine_document_path").Where("id = ?", doc.ID).Scan(&reloaded).Error)
	assert.NotEmpty(t, reloaded.CedulaDocumentPath)
	assert.NotEmpty(t, reloaded.IneDocumentPath)
}
