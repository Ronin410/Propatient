package server_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// doRequest arma y ejecuta una petición HTTP contra el router real (mismo
// middleware, mismas rutas que en producción), sin levantar un servidor TCP.
func doRequest(t *testing.T, router *gin.Engine, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// decodeJSON parsea el body de la respuesta a un map genérico.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

// decodeJSONList parsea el body de la respuesta (un array JSON) al puntero dado.
func decodeJSONList(t *testing.T, w *httptest.ResponseRecorder, out any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), out))
}
