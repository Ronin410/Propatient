// Package middleware agrupa middlewares de Gin de propósito general
// (no específicos de auth/billing, que ya tienen su propio paquete).
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter limita solicitudes por IP con ventanas fijas, guardadas en
// memoria del propio proceso. No requiere Redis ni ninguna dependencia
// externa — alcanza para la única instancia del backend que corre hoy en
// Render. Si el backend llega a escalarse a varias instancias, esto debe
// migrar a un store compartido (Redis) para que el límite sea global y no
// por instancia.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitorState
	max      int
	window   time.Duration
}

type visitorState struct {
	count      int
	windowEnds time.Time
}

// NewRateLimiter crea un limitador que permite hasta max solicitudes por
// IP dentro de cada ventana de duración window.
func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitorState),
		max:      max,
		window:   window,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[key]
	if !exists || now.After(v.windowEnds) {
		rl.visitors[key] = &visitorState{count: 1, windowEnds: now.Add(rl.window)}
		return true
	}
	if v.count >= rl.max {
		return false
	}
	v.count++
	return true
}

// cleanupLoop borra periódicamente las IPs cuya ventana ya expiró, para que
// el mapa no crezca sin límite en un proceso de larga duración.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, v := range rl.visitors {
			if now.After(v.windowEnds) {
				delete(rl.visitors, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware devuelve el gin.HandlerFunc que aplica el límite a la IP del
// cliente de cada solicitud (c.ClientIP(), que ya respeta X-Forwarded-For
// detrás del proxy de Render).
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Demasiadas solicitudes. Espera unos minutos e intenta de nuevo.",
			})
			return
		}
		c.Next()
	}
}
