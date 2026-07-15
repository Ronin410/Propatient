// Package testutil provee helpers compartidos para los tests de integración
// del backend: conexión a una base de datos de pruebas real (Postgres) y
// creación de doctores/tokens de prueba.
package testutil

import (
	"os"
	"testing"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/billing"
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
		&models.Staff{},
	); err != nil {
		t.Fatalf("Error en AutoMigrate de la DB de pruebas: %v", err)
	}

	// Limpieza total antes de cada test para que no arrastre datos de corridas anteriores.
	if err := db.Exec(
		"TRUNCATE TABLE doctor_patients, medical_documents, appointments, medical_histories, patients, doctor_templates, staffs, doctors RESTART IDENTITY CASCADE",
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

	// SubscriptionStatus/TrialEndsAt explícitos: reflejan lo que hace
	// auth.RegisterDoctor/GoogleLoginHandler de verdad. Sin esto, los
	// doctores de prueba quedan con TrialEndsAt nil y
	// billing.RequireActiveSubscription los bloquea con 402 en cualquier
	// test que use rutas protegidas (ver internal/billing/middleware.go).
	trialEndsAt := time.Now().UTC().Add(billing.TrialDuration)
	doctor := models.Doctor{
		Username:           username,
		PasswordHash:       string(hashed),
		FullName:           "Dr. Test " + username,
		Email:              username + "@test.local",
		SubscriptionStatus: "trialing",
		TrialEndsAt:        &trialEndsAt,
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

// CreateTestStaff inserta una cuenta de personal de prueba con contraseña
// conocida, vinculada al doctor indicado.
func CreateTestStaff(t *testing.T, db *gorm.DB, doctorID uint, email, password string) models.Staff {
	t.Helper()

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("Error al hashear password de prueba: %v", err)
	}

	staff := models.Staff{
		DoctorID:     doctorID,
		FullName:     "Personal de prueba",
		Email:        email,
		PasswordHash: string(hashed),
		Active:       true,
		PasswordSet:  true,
	}
	if err := db.Create(&staff).Error; err != nil {
		t.Fatalf("Error al crear personal de prueba: %v", err)
	}
	return staff
}

// TokenForStaff genera un JWT real de personal (mismo doctorID del dueño
// del consultorio, con role "STAFF") para el registro de Staff indicado.
func TokenForStaff(t *testing.T, doctorID, staffID uint, email string) string {
	t.Helper()
	token, err := auth.GenerateStaffToken(doctorID, staffID, email)
	if err != nil {
		t.Fatalf("Error al generar JWT de personal de prueba: %v", err)
	}
	return token
}
