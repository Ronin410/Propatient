package auth

import (
	"net/http"
	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func LoginHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest

		// 1. Validar formato del JSON de entrada
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
			return
		}

		var doctor models.Doctor
		// 2. Buscar doctor por username en la base de datos
		if err := db.Where("username = ?", req.Username).First(&doctor).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario o contraseña incorrectos"})
			return
		}

		// 3. Verificar Password usando Bcrypt
		err := bcrypt.CompareHashAndPassword([]byte(doctor.PasswordHash), []byte(req.Password))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario o contraseña incorrectos"})
			return
		}

		// 4. Generar Token (PASANDO EL ID Y EL USERNAME)
		// IMPORTANTE: doctor.ID es el que usaremos en las relaciones de la DB
		token, err := GenerateToken(doctor.ID, doctor.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar la sesión"})
			return
		}

		// 5. Respuesta exitosa
		c.JSON(http.StatusOK, gin.H{
			"token":    token,
			"fullName": doctor.FullName,
			"username": doctor.Username,
		})
	}
}

func RegisterDoctor(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username  string `json:"username" binding:"required"`
			Password  string `json:"password" binding:"required"`
			FullName  string `json:"full_name" binding:"required"`
			Specialty string `json:"specialty"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Datos incompletos"})
			return
		}

		// Encriptar contraseña
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

		doctor := models.Doctor{
			Username:         req.Username,
			PasswordHash:     string(hashedPassword),
			FullName:         req.FullName,
			MedicalSpecialty: req.Specialty,
		}

		if err := db.Create(&doctor).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "El nombre de usuario ya está en uso"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Doctor registrado exitosamente"})
	}
}
