package server_test

import (
	"net/http"
	"testing"

	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
)

func TestLogin_Success(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testutil.CreateTestDoctor(t, db, "drtest_login_ok", "supersecret123")
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodPost, "/api/auth/login", "", map[string]any{
		"username": "drtest_login_ok", "password": "supersecret123",
	})

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.NotEmpty(t, resp["token"])
}

func TestLogin_WrongPassword(t *testing.T) {
	db := testutil.SetupTestDB(t)
	testutil.CreateTestDoctor(t, db, "drtest_login_bad", "supersecret123")
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodPost, "/api/auth/login", "", map[string]any{
		"username": "drtest_login_bad", "password": "wrongpassword",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_UnknownUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodPost, "/api/auth/login", "", map[string]any{
		"username": "no-existe", "password": "lo-que-sea",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestProtectedRoute_NoToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodGet, "/api/patients", "", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestProtectedRoute_InvalidToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodGet, "/api/patients", "not-a-real-token", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestProtectedRoute_ValidToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doctor := testutil.CreateTestDoctor(t, db, "drtest_valid_token", "supersecret123")
	router := server.NewRouter(db)
	token := testutil.TokenFor(t, doctor.ID, doctor.Username)

	w := doRequest(t, router, http.MethodGet, "/api/patients", token, nil)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRegister_RequiresEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	// Antes del fix, este endpoint no exigía email, lo que hacía que el
	// segundo registro chocara por el unique constraint sobre email vacío.
	w := doRequest(t, router, http.MethodPost, "/api/auth/register", "", map[string]any{
		"username": "newdoc_no_email", "password": "supersecret123", "full_name": "Dr Nuevo",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegister_ThenLogin(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodPost, "/api/auth/register", "", map[string]any{
		"username": "newdoc_ok", "password": "supersecret123", "full_name": "Dr Nuevo", "email": "nuevo@test.local",
	})
	assert.Equal(t, http.StatusCreated, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/auth/login", "", map[string]any{
		"username": "newdoc_ok", "password": "supersecret123",
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthCheck(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodGet, "/api/health", "", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON(t, w)
	assert.Equal(t, "conectada", resp["db"])
}
