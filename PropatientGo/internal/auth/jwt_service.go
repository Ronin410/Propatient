package auth

import (
	"errors"
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

// GenerateStaffToken firma el JWT de una cuenta de personal (secretaria/
// asistente). A propósito usa el mismo claim "userId" que GenerateToken,
// pero con el ID del DOCTOR dueño del consultorio (no el del propio
// registro de Staff): así todos los handlers existentes, que ya filtran
// todo por "doctorID" del contexto, funcionan sin ningún cambio para el
// personal. "staffId" y "role": "STAFF" identifican la sesión como
// personal para el middleware (ver RequireDoctorRole) y para auditoría.
func GenerateStaffToken(doctorID, staffID uint, email string) (string, error) {
	secret := os.Getenv("JWT_SECRET")

	claims := jwt.MapClaims{
		"userId":  doctorID,
		"staffId": staffID,
		"sub":     email,
		"role":    "STAFF",
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateStaffSelectionToken firma un JWT de muy corta vida (5 min) que
// prueba que el correo/contraseña de una cuenta de personal ya se
// validaron, para el caso de alguien con acceso activo a más de un
// consultorio (ver StaffLoginHandler/SelectStaffDoctorHandler). No da
// acceso a ninguna ruta protegida por sí solo — AuthorizeJWT lo
// rechazaría (no lleva "userId") — solo sirve para el segundo paso de
// "elige con cuál consultorio quieres entrar".
func GenerateStaffSelectionToken(staffID uint, email string) (string, error) {
	secret := os.Getenv("JWT_SECRET")

	claims := jwt.MapClaims{
		"staffId": staffID,
		"sub":     email,
		"role":    "STAFF_SELECT_DOCTOR",
		"exp":     time.Now().Add(5 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateStaffSelectionToken valida un token emitido por
// GenerateStaffSelectionToken y devuelve el staffID que prueba.
func ValidateStaffSelectionToken(tokenString string) (uint, error) {
	token, err := ValidateToken(tokenString)
	if err != nil || !token.Valid {
		return 0, errors.New("token de selección inválido o vencido")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["role"] != "STAFF_SELECT_DOCTOR" {
		return 0, errors.New("el token no es de selección de consultorio")
	}
	staffIDFloat, ok := claims["staffId"].(float64)
	if !ok {
		return 0, errors.New("token de selección mal formado")
	}
	return uint(staffIDFloat), nil
}

// GenerateSuperAdminToken firma el JWT de una cuenta interna de ProPatient
// (ver models.SuperAdmin). A propósito NO lleva "userId": estas cuentas no
// pertenecen a ningún consultorio y no deben poder colarse en ninguna ruta
// gated por doctorID. Se valida con AuthorizeSuperAdminJWT, no con
// AuthorizeJWT.
func GenerateSuperAdminToken(adminID uint, username string) (string, error) {
	secret := os.Getenv("JWT_SECRET")

	claims := jwt.MapClaims{
		"adminId": adminID,
		"sub":     username,
		"role":    "SUPERADMIN",
		"exp":     time.Now().Add(time.Hour * 12).Unix(),
		"iat":     time.Now().Unix(),
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
