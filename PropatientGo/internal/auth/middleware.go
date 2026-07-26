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

		// "role" identifica si la sesión es del doctor dueño del consultorio
		// ("MEDICO") o de una cuenta de personal ("STAFF"). Los tokens
		// emitidos antes de esta funcionalidad no traen el claim; se asume
		// "MEDICO" para no romper sesiones existentes.
		role, _ := claims["role"].(string)
		if role == "" {
			role = "MEDICO"
		}
		c.Set("role", role)

		if staffIDFloat, ok := claims["staffId"].(float64); ok {
			c.Set("staffId", uint(staffIDFloat))
		}

		// 5. Continuar al siguiente Handler
		c.Next()
	}
}

// AuthorizeSuperAdminJWT protege las rutas internas de administración
// (/api/admin/*). Deliberadamente separado de AuthorizeJWT: un token de
// SuperAdmin nunca lleva "userId" (no pertenece a ningún consultorio), así
// que reusar AuthorizeJWT lo rechazaría igual, pero por la razón
// equivocada — esto lo rechaza explícitamente si el rol no es
// "SUPERADMIN", sin importar si el token es técnicamente válido.
func AuthorizeSuperAdminJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		const BearerSchema = "Bearer "
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, BearerSchema) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Acceso denegado. Token no proporcionado.",
			})
			return
		}

		tokenString := authHeader[len(BearerSchema):]
		token, err := ValidateToken(tokenString)
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Sesión inválida o expirada. Por favor, inicie sesión de nuevo.",
			})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token inválido"})
			return
		}

		role, _ := claims["role"].(string)
		if role != "SUPERADMIN" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Esta acción requiere una cuenta de administrador."})
			return
		}

		adminIDFloat, ok := claims["adminId"].(float64)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token mal formado: falta ID"})
			return
		}
		c.Set("superAdminId", uint(adminIDFloat))

		c.Next()
	}
}

// RequireDoctorRole bloquea con 403 cualquier ruta a la que una cuenta de
// personal ("STAFF") intente acceder. Debe montarse DESPUÉS de
// AuthorizeJWT() (necesita que "role" ya esté en el contexto). Se usa en
// las rutas de historial clínico, contenido de consultas, perfil del
// doctor y gestión del propio personal — todo lo que el personal no debe
// poder ver ni tocar.
func RequireDoctorRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "MEDICO" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Esta acción solo está disponible para la cuenta del doctor.",
			})
			return
		}
		c.Next()
	}
}
