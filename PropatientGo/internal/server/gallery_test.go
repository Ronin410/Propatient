package server_test

import (
	"net/http"
	"strconv"
	"testing"

	"propatient-api/internal/billing"
	"propatient-api/internal/geocoding"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGallery_UploadListDelete cubre el ciclo de vida completo: subir una
// foto, verla en el listado con URL firmada, y borrarla.
func TestGallery_UploadListDelete(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_gallery", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doMultipartRequest(t, router, http.MethodPost, "/api/doctor/gallery", token,
		nil, "image", "foto1.png", minimalPNGBytes)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	uploaded := decodeJSON(t, w)
	assert.Contains(t, uploaded["imagePath"], "mock-presigned")
	imageID := int(uploaded["id"].(float64))

	w = doRequest(t, router, http.MethodGet, "/api/doctor/gallery", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var images []map[string]any
	decodeJSONList(t, w, &images)
	require.Len(t, images, 1)

	w = doRequest(t, router, http.MethodDelete, "/api/doctor/gallery/"+strconv.Itoa(imageID), token, nil)
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/doctor/gallery", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	decodeJSONList(t, w, &images)
	assert.Len(t, images, 0)
}

// TestGallery_EnforcesMaxImages confirma que no se puede pasar del límite
// de fotos configurado (maxGalleryImages = 8).
func TestGallery_EnforcesMaxImages(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_gallery_max", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	for i := 0; i < 8; i++ {
		w := doMultipartRequest(t, router, http.MethodPost, "/api/doctor/gallery", token,
			nil, "image", "foto.png", minimalPNGBytes)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	}

	w := doMultipartRequest(t, router, http.MethodPost, "/api/doctor/gallery", token,
		nil, "image", "foto9.png", minimalPNGBytes)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestGallery_OnlyDoctorAccess confirma que el personal no puede gestionar
// la galería (a diferencia del horario, esta sí es exclusiva del doctor).
func TestGallery_OnlyDoctorAccess(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_gallery_staff", "password123")
	staff := testutil.CreateTestStaff(t, db, doc.ID, "personal_gallery@test.local", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodGet, "/api/doctor/gallery", staffToken, nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestGallery_ShownOnPublicProfile confirma que las fotos subidas aparecen
// en el perfil público del doctor.
func TestGallery_ShownOnPublicProfile(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockStorage := newMockStorageClient()
	router := server.NewRouterWithDeps(db, googlecalendar.Config{}, nil, mockStorage, billing.Config{}, nil, geocoding.NewClient(), nil, nil)

	doc := testutil.CreateTestDoctor(t, db, "doc_gallery_public", "password123")
	require.NoError(t, db.Model(&doc).Updates(map[string]any{"public_listed": true, "public_slug": "dr-gallery-public"}).Error)
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doMultipartRequest(t, router, http.MethodPost, "/api/doctor/gallery", token,
		nil, "image", "foto1.png", minimalPNGBytes)
	require.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/public/doctors/dr-gallery-public", "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	gallery, ok := body["galleryImages"].([]any)
	require.True(t, ok)
	assert.Len(t, gallery, 1)
}
