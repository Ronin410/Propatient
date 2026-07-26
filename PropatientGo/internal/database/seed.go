package database

import (
	"fmt"
	"log"
	"os"
	"propatient-api/internal/billing"
	"propatient-api/internal/models"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedDatabase crea el doctor de prueba "medico"/"12345" (ver PROJECT_SPECS.md),
// pero SOLO si ENABLE_TEST_SEED="true" — nunca por defecto. Antes se creaba
// en cada arranque sin condición, incluido producción: una cuenta con
// contraseña conocida públicamente (está documentada en este mismo repo)
// viviendo en la base de datos real es una puerta trasera real. En local,
// docker-compose.yml ya define ENABLE_TEST_SEED=true por defecto, así que
// el flujo de desarrollo de siempre sigue funcionando sin que el
// desarrollador tenga que hacer nada distinto.
func SeedDatabase(db *gorm.DB) {
	if os.Getenv("ENABLE_TEST_SEED") != "true" {
		return
	}

	var count int64
	db.Model(&models.Doctor{}).Where("username = ?", "medico").Count(&count)

	if count == 0 {
		// Encriptar contraseña "12345"
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("12345"), bcrypt.DefaultCost)
		if err != nil {
			log.Fatal("Error al encriptar contraseña de semilla")
		}

		trialEndsAt := time.Now().UTC().Add(billing.TrialDuration)
		medico := models.Doctor{
			FullName:           "Dr. Alejandro ProPatient",
			Username:           "medico",
			PasswordHash:       string(hashedPassword),
			Email:              "medico@propatient.com",
			MedicalSpecialty:   "Medicina General",
			SubscriptionStatus: "trialing",
			TrialEndsAt:        &trialEndsAt,
		}

		if err := db.Create(&medico).Error; err != nil {
			log.Printf("Error al crear doctor de prueba: %v", err)
		} else {
			fmt.Println("✅ Doctor de prueba 'medico' inicializado con éxito.")
		}
	}
}

// SeedSuperAdmin crea la primera cuenta de administrador (panel de
// aprobación de cédula) a partir de SUPERADMIN_USERNAME/SUPERADMIN_PASSWORD,
// solo si todavía no existe ninguna. A propósito no hay usuario/contraseña
// por defecto (a diferencia del doctor de prueba "medico"): un admin con
// contraseña conocida en cada despliegue, incluido producción, sería una
// puerta trasera real. Sin esas dos variables definidas, simplemente no se
// crea ninguna cuenta y el panel queda inaccesible hasta que se configuren.
func SeedSuperAdmin(db *gorm.DB) {
	var count int64
	db.Model(&models.SuperAdmin{}).Count(&count)
	if count > 0 {
		return
	}

	username := os.Getenv("SUPERADMIN_USERNAME")
	password := os.Getenv("SUPERADMIN_PASSWORD")
	if username == "" || password == "" {
		log.Println("⚠️ SUPERADMIN_USERNAME/SUPERADMIN_PASSWORD no configuradas — no se creó ninguna cuenta de administrador todavía.")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error al encriptar contraseña del administrador: %v", err)
		return
	}

	admin := models.SuperAdmin{Username: username, PasswordHash: string(hashedPassword)}
	if err := db.Create(&admin).Error; err != nil {
		log.Printf("Error al crear la cuenta de administrador: %v", err)
		return
	}
	fmt.Printf("✅ Cuenta de administrador '%s' inicializada con éxito.\n", username)
}
