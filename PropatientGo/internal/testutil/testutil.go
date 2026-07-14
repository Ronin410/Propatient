// Package testutil provee helpers compartidos para los tests de integración
// del backend: conexión a una base de datos de pruebas real (Postgres) y
// creación de doctores/tokens de prueba.
package testutil

import (
	"os"
	"testing"

	"propatient-api/internal/auth"
	"propatient-api/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const TestJWTSecret = "test-secret-not-for-production"

// SetupTestDB conecta a una base de datos de pruebas (TEST_DATABASE_URL, o
// un Postgres local por defecto), corre las migraciones y limpia todas las
// tablas antes de cada test. Si no hay una DB de pruebas disponible, salta
// el test con t.Skip() en vez de fallar — así "go test ./..." no rompe en
// una máquina que no tenga Postgres configurado para pruebas.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/propatient_test?sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("Saltando: no se pudo conectar a la DB de pruebas (%s). "+
			"Define TEST_DATABASE_URL para correr estos tests contra tu propio Postgres. Error: %v", dsn, err)
	}

	if err := db.AutoMigrate(
		&models.Doctor{}, &models.Patient{}, &models.MedicalHistory{},
		&models.Appointment{}, &models.MedicalDocument{}, &models.DoctorTemplate{},
	); err != nil {
		t.Fatalf("Error en AutoMigrate de la DB de pruebas: %v", err)
	}

	// Limpieza total antes de cada test para que no arrastre datos de corridas anteriores.
	if err := db.Exec(
		"TRUNCATE TABLE doctor_patients, medical_documents, appointments, medical_histories, patients, doctor_templates, doctors RESTART IDENTITY CASCADE",
	).Error; err != nil {
		t.Fatalf("Error al limpiar la DB de pruebas: %v", err)
	}

	os.Setenv("JWT_SECRET", TestJWTSecret)

	return db
}

// CreateTestDoctor inserta un doctor de prueba con contraseña conocida.
func CreateTestDoctor(t *testing.T, db *gorm.DB, username, password string) models.Doctor {
	t.Helper()

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Error al hashear password de prueba: %v", err)
	}

	doctor := models.Doctor{
		Username:     username,
		PasswordHash: string(hashed),
		FullName:     "Dr. Test " + username,
		Email:        username + "@test.local",
	}
	if err := db.Create(&doctor).Error; err != nil {
		t.Fatalf("Error al crear doctor de prueba: %v", err)
	}
	return doctor
}

// TokenFor genera un JWT real (misma función que usa el login de verdad)
// para el doctor indicado.
func TokenFor(t *testing.T, doctorID uint, username string) string {
	t.Helper()
	token, err := auth.GenerateToken(doctorID, username)
	if err != nil {
		t.Fatalf("Error al generar JWT de prueba: %v", err)
	}
	return token
}
