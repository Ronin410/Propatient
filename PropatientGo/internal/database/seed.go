package database

import (
	"fmt"
	"log"
	"propatient-api/internal/billing"
	"propatient-api/internal/models"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedDatabase replica la lógica de DataInitializer.java
func SeedDatabase(db *gorm.DB) {
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
