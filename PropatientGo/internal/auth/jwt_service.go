package auth

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateToken ahora recibe el ID y el username para que el sistema sea rastreable
func GenerateToken(doctorID uint, username string) (string, error) {
	secret := os.Getenv("JWT_SECRET")

	claims := jwt.MapClaims{
		"userId": doctorID, // <--- CRUCIAL: Esto es lo que el Middleware leerá
		"sub":    username,
		"role":   "MEDICO",
		"exp":    time.Now().Add(time.Hour * 24).Unix(),
		"iat":    time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken se mantiene igual, es una función de apoyo para el Middleware
func ValidateToken(tokenString string) (*jwt.Token, error) {
	secret := os.Getenv("JWT_SECRET")
	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validar que el método de firma sea el esperado
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
}