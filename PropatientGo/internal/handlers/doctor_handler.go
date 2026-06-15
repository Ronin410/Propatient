package handlers

import (
	"net/http"
	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// GetCurrentDoctor devuelve el perfil del doctor autenticado
func GetCurrentDoctor(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		// GORM ya tiene json:"-" en PasswordHash en tu modelo,
		// por lo que no se enviará la contraseña.
		c.JSON(http.StatusOK, doctor)
	}
}

// UpdateCurrentDoctor permite actualizar especialidad, teléfono o contraseña
func UpdateCurrentDoctor(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		// Estructura temporal para recibir los datos del update
		var updateReq struct {
			FullName         string `json:"full_name"`
			MedicalSpecialty string `json:"medical_specialty"`
			Phone            string `json:"phone"`
			LicenseNumber    string `json:"license_number"`
			Password         string `json:"password"` // Opcional
		}

		if err := c.ShouldBindJSON(&updateReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
			return
		}

		// Actualizamos campos básicos
		doctor.FullName = updateReq.FullName
		doctor.MedicalSpecialty = updateReq.MedicalSpecialty
		doctor.Phone = updateReq.Phone
		doctor.LicenseNumber = updateReq.LicenseNumber

		// Si envió una nueva contraseña, la encriptamos
		if updateReq.Password != "" {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(updateReq.Password), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al procesar la contraseña"})
				return
			}
			doctor.PasswordHash = string(hashedPassword)
		}

		if err := db.Save(&doctor).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar el perfil"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Perfil actualizado con éxito"})
	}
}
