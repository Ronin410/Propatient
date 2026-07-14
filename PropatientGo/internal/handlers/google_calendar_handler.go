package handlers

import (
	"net/http"
	"os"
	"strings"

	"propatient-api/internal/auth"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// frontendRedirectBase toma la primera URL de FRONTEND_URL (puede venir
// separada por comas, igual que en la config de CORS) para redirigir al
// navegador tras el callback de Google.
func frontendRedirectBase() string {
	raw := os.Getenv("FRONTEND_URL")
	first := strings.TrimSpace(strings.Split(raw, ",")[0])
	first = strings.TrimSuffix(first, "/")
	if first == "" {
		return "http://localhost:5173"
	}
	if !strings.HasPrefix(first, "http://") && !strings.HasPrefix(first, "https://") {
		first = "https://" + first
	}
	return first
}

// ConnectGoogleCalendar arma la URL de consentimiento de Google y la
// devuelve como JSON (el frontend hace window.location.href = url). No
// redirige directamente porque esta ruta va protegida por JWT vía header
// Authorization, que un redirect de navegador no puede enviar.
func ConnectGoogleCalendar(cfg googlecalendar.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.IsConfigured() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "La integración con Google Calendar no está configurada en el servidor."})
			return
		}

		doctorID := c.MustGet("doctorID").(uint)

		// Reutilizamos el emisor de JWT existente como "state": ya sabe
		// firmar/verificar, y el callback (sin sesión del frontend) solo
		// necesita el claim "userId" para saber a qué doctor pertenece el
		// código de autorización que Google le manda.
		state, err := auth.GenerateToken(doctorID, "")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo iniciar la conexión con Google Calendar"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": cfg.AuthURL(state)})
	}
}

// GoogleCalendarCallback es el endpoint público al que Google redirige tras
// el consentimiento. Guarda el refresh_token del doctor y regresa al
// navegador a la pantalla de Perfil del frontend.
func GoogleCalendarCallback(db *gorm.DB, cfg googlecalendar.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		redirectBase := frontendRedirectBase()

		if errParam := c.Query("error"); errParam != "" {
			c.Redirect(http.StatusFound, redirectBase+"/profile?calendar=error")
			return
		}

		state := c.Query("state")
		code := c.Query("code")
		if state == "" || code == "" {
			c.Redirect(http.StatusFound, redirectBase+"/profile?calendar=error")
			return
		}

		token, err := auth.ValidateToken(state)
		if err != nil || !token.Valid {
			c.Redirect(http.StatusFound, redirectBase+"/profile?calendar=error")
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.Redirect(http.StatusFound, redirectBase+"/profile?calendar=error")
			return
		}
		userIDFloat, ok := claims["userId"].(float64)
		if !ok {
			c.Redirect(http.StatusFound, redirectBase+"/profile?calendar=error")
			return
		}
		doctorID := uint(userIDFloat)

		refreshToken, err := cfg.Exchange(c.Request.Context(), code)
		if err != nil {
			c.Redirect(http.StatusFound, redirectBase+"/profile?calendar=error")
			return
		}

		if err := db.Model(&models.Doctor{}).Where("id = ?", doctorID).
			Update("google_calendar_refresh_token", refreshToken).Error; err != nil {
			c.Redirect(http.StatusFound, redirectBase+"/profile?calendar=error")
			return
		}

		c.Redirect(http.StatusFound, redirectBase+"/profile?calendar=connected")
	}
}

// DisconnectGoogleCalendar borra el refresh token guardado del doctor.
func DisconnectGoogleCalendar(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		if err := db.Model(&models.Doctor{}).Where("id = ?", doctorID).
			Update("google_calendar_refresh_token", "").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo desconectar Google Calendar"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Google Calendar desconectado"})
	}
}
