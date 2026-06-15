package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthorizeJWT protege las rutas verificando el token Bearer
func AuthorizeJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. SOLUCIÓN AL CORS:
		// Si es OPTIONS, dejamos que el middleware de CORS de Gin maneje la respuesta.
		// AbortWithStatus(204) asegura que no se ejecute el resto de este middleware.
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		// 2. Obtener y validar formato del Header
		const BearerSchema = "Bearer "
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" || !strings.HasPrefix(authHeader, BearerSchema) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Acceso denegado. Token no proporcionado.",
			})
			return
		}

		// 3. Extraer y Validar el Token
		tokenString := authHeader[len(BearerSchema):]
		token, err := ValidateToken(tokenString)

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Sesión inválida o expirada. Por favor, inicie sesión de nuevo.",
			})
			return
		}

		// 4. Extraer claims y setear doctorID en el contexto
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token inválido"})
			return
		}

		// Importante: jwt-go parsea números de JSON como float64
		val, exists := claims["userId"]
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token mal formado: falta ID"})
			return
		}

		// Conversión segura de float64 (JWT) a uint (GORM)
		doctorIDFloat, ok := val.(float64)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "ID de usuario en formato inválido"})
			return
		}

		// Guardamos el ID en el contexto para usarlo en los handlers
		// Ejemplo en handler: userID := c.MustGet("doctorID").(uint)
		c.Set("doctorID", uint(doctorIDFloat))

		// 5. Continuar al siguiente Handler
		c.Next()
	}
}
