package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newTestRouter(rl *RateLimiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ping", rl.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func doRequest(r *gin.Engine, remoteAddr string) int {
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = remoteAddr
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestRateLimiter_AllowsUpToMaxThenBlocks(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	r := newTestRouter(rl)

	for i := 0; i < 3; i++ {
		if code := doRequest(r, "203.0.113.5:1234"); code != http.StatusOK {
			t.Fatalf("solicitud %d: esperaba 200, obtuve %d", i+1, code)
		}
	}

	if code := doRequest(r, "203.0.113.5:1234"); code != http.StatusTooManyRequests {
		t.Fatalf("esperaba 429 tras exceder el límite, obtuve %d", code)
	}
}

func TestRateLimiter_TracksEachIPSeparately(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	r := newTestRouter(rl)

	if code := doRequest(r, "203.0.113.5:1234"); code != http.StatusOK {
		t.Fatalf("IP A: esperaba 200, obtuve %d", code)
	}
	if code := doRequest(r, "203.0.113.5:1234"); code != http.StatusTooManyRequests {
		t.Fatalf("IP A repetida: esperaba 429, obtuve %d", code)
	}
	if code := doRequest(r, "198.51.100.9:1234"); code != http.StatusOK {
		t.Fatalf("IP B distinta: esperaba 200 (límite independiente), obtuve %d", code)
	}
}

func TestRateLimiter_ResetsAfterWindowExpires(t *testing.T) {
	rl := NewRateLimiter(1, 30*time.Millisecond)
	r := newTestRouter(rl)

	if code := doRequest(r, "203.0.113.5:1234"); code != http.StatusOK {
		t.Fatalf("primera solicitud: esperaba 200, obtuve %d", code)
	}
	if code := doRequest(r, "203.0.113.5:1234"); code != http.StatusTooManyRequests {
		t.Fatalf("segunda solicitud inmediata: esperaba 429, obtuve %d", code)
	}

	time.Sleep(50 * time.Millisecond)

	if code := doRequest(r, "203.0.113.5:1234"); code != http.StatusOK {
		t.Fatalf("tras expirar la ventana: esperaba 200, obtuve %d", code)
	}
}
