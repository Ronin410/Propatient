package server_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"propatient-api/internal/billing"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doMultipartRequest arma y ejecuta una petición multipart/form-data (subida
// de archivos), a diferencia de doRequest que solo maneja JSON.
func doMultipartRequest(t *testing.T, router http.Handler, method, path, token string, fields map[string]string, fileField, fileName string, fileContent []byte) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for k, v := range fields {
		require.NoError(t, writer.WriteField(k, v))
	}
	if fileField != "" {
		part, err := writer.CreateFormFile(fileField, fileName)
		require.NoError(t, err)
		_, err = part.Write(fileContent)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestStorage_AvatarUpload_ReturnsPresignedURLOnRead(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil)

	doc := testutil.CreateTestDoctor(t, db, "doctor_storage_avatar", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doMultipartRequest(t, router, http.MethodPut, "/api/doctor/me", token,
		map[string]string{"fullName": "Dr. Avatar"}, "avatar", "foto.png", []byte("fake-png-bytes"))
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)

	avatarURL, _ := body["avatarUrl"].(string)
	assert.Contains(t, avatarURL, "https://mock-presigned.example.com/profiles/doc_", "la respuesta debe traer la URL ya firmada, no la key cruda")
	assert.Equal(t, 1, mockStorage.savedKeyCount())

	// GET /doctor/me también debe devolver la URL firmada (no la key guardada en la BD).
	w = doRequest(t, router, http.MethodGet, "/api/doctor/me", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body = decodeJSON(t, w)
	avatarURL, _ = body["avatarUrl"].(string)
	assert.Contains(t, avatarURL, "https://mock-presigned.example.com/profiles/doc_")
}

func TestStorage_DocumentUpload_PresignedOnAppointmentDetail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil)

	doc := testutil.CreateTestDoctor(t, db, "doctor_storage_doc", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Storage", "email": "storage@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	w = doMultipartRequest(t, router, http.MethodPost, "/api/appointments/"+strconv.Itoa(apptID)+"/upload-document", token,
		map[string]string{"isPrescription": "false"}, "files", "estudio.pdf", []byte("fake-pdf-bytes"))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mockStorage.savedKeyCount())

	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(apptID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	documents, ok := body["documents"].([]any)
	require.True(t, ok)
	require.Len(t, documents, 1)
	doc0 := documents[0].(map[string]any)
	filePath, _ := doc0["file_path"].(string)
	assert.Contains(t, filePath, "https://mock-presigned.example.com/documents/", "el documento debe verse con una URL firmada, no la key cruda")
}

func TestStorage_RecipePDF_PresignedOnAppointmentAndPatientHistory(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil)

	doc := testutil.CreateTestDoctor(t, db, "doctor_storage_recipe", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Recipe", "email": "recipe_storage@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	w = doMultipartRequest(t, router, http.MethodPost, "/api/appointments/"+strconv.Itoa(apptID)+"/save-recipe-pdf", token,
		nil, "recipe_pdf", "receta.pdf", []byte("fake-recipe-bytes"))
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(apptID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	recipePath, _ := body["recipePdfPath"].(string)
	assert.Contains(t, recipePath, "https://mock-presigned.example.com/recipes/receta_")

	// La misma cita, vista desde el historial del paciente, también debe
	// traer la receta ya firmada (usada por el botón "Imprimir Receta").
	w = doRequest(t, router, http.MethodGet, "/api/patients/"+strconv.Itoa(patientID)+"/history", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	historyBody := decodeJSON(t, w)
	appointments, ok := historyBody["appointments"].([]any)
	require.True(t, ok)
	require.Len(t, appointments, 1)
	apptFromHistory := appointments[0].(map[string]any)
	recipePathFromHistory, _ := apptFromHistory["recipePdfPath"].(string)
	assert.Contains(t, recipePathFromHistory, "https://mock-presigned.example.com/recipes/receta_")
}
