package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"propatient-api/internal/models"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetCurrentDoctor devuelve el perfil del doctor autenticado
// doctorWithCalendarStatus agrega un campo calculado (nunca el token en sí)
// para que el frontend sepa si mostrar "Conectar" o "Desconectar".
type doctorWithCalendarStatus struct {
	models.Doctor
	GoogleCalendarConnected bool `json:"googleCalendarConnected"`
}

func GetCurrentDoctor(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		// GORM ya tiene json:"-" en PasswordHash y en el refresh token de
		// Google Calendar en el modelo, por lo que nunca se envían.
		c.JSON(http.StatusOK, doctorWithCalendarStatus{
			Doctor:                  doctor,
			GoogleCalendarConnected: doctor.GoogleCalendarRefreshToken != "",
		})
	}
}

func UpdateCurrentDoctor(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		// 1. Crear la carpeta para almacenar las imágenes si no existe (ej: ./uploads/profiles)
		uploadDir := "./uploads/profiles"
		if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al preparar el almacenamiento en el servidor"})
			return
		}

		// 2. Procesar y guardar el AVATAR (Foto de perfil) si viene en la petición
		avatarFile, err := c.FormFile("avatar")
		if err == nil && avatarFile != nil {
			// Generar nombre único usando timestamp para evitar sobreescritura
			ext := filepath.Ext(avatarFile.Filename)
			avatarName := fmt.Sprintf("doc_%d_avatar_%d%s", doctorID, time.Now().Unix(), ext)
			avatarPath := filepath.Join(uploadDir, avatarName)

			// Guardar el archivo físicamente en la carpeta del backend
			if err := c.SaveUploadedFile(avatarFile, avatarPath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar la foto de perfil"})
				return
			}
			// Guardar la URL o ruta estática relativa en la base de datos
			// Nota: Si configuraste archivos estáticos en Gin, la ruta pública limpia sería: "/uploads/profiles/" + avatarName
			doctor.AvatarUrl = "/uploads/profiles/" + avatarName
		}

		// 3. Procesar y guardar el LOGO de la clínica si viene en la petición
		logoFile, err := c.FormFile("logo")
		if err == nil && logoFile != nil {
			ext := filepath.Ext(logoFile.Filename)
			logoName := fmt.Sprintf("doc_%d_logo_%d%s", doctorID, time.Now().Unix(), ext)
			logoPath := filepath.Join(uploadDir, logoName)

			if err := c.SaveUploadedFile(logoFile, logoPath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar el logo de la clínica"})
				return
			}
			doctor.LogoUrl = "/uploads/profiles/" + logoName
		}

		// 4. Leer los campos de texto regulares utilizando PostForm (ya que vienen en multipart)
		doctor.FullName = c.PostForm("fullName")
		doctor.MedicalSpecialty = c.PostForm("medicalSpecialty")
		doctor.Phone = c.PostForm("phone")
		doctor.University = c.PostForm("university")
		doctor.Address = c.PostForm("address")
		doctor.RecipeLegend = c.PostForm("recipeLegend")
		doctor.Resume = c.PostForm("resume")

		// Campos opcionales si también los mandas
		if rfc := c.PostForm("rfc"); rfc != "" {
			doctor.RFC = rfc
		}
		if curp := c.PostForm("curp"); curp != "" {
			doctor.CURP = curp
		}
		if license := c.PostForm("licenseNumber"); license != "" {
			doctor.LicenseNumber = license
		}

		// 5. Persistir los cambios en la Base de Datos
		if err := db.Save(&doctor).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar el perfil"})
			return
		}

		// 6. IMPORTANTE: Devolver el objeto actualizado o las nuevas URLs
		// para que el FrontEnd las mapee de inmediato en el estado de React
		c.JSON(http.StatusOK, gin.H{
			"message":          "Perfil actualizado con éxito",
			"avatarUrl":        doctor.AvatarUrl,
			"logoUrl":          doctor.LogoUrl,
			"fullName":         doctor.FullName,
			"medicalSpecialty": doctor.MedicalSpecialty,
			"phone":            doctor.Phone,
			"university":       doctor.University,
			"address":          doctor.Address,
			"recipeLegend":     doctor.RecipeLegend,
			"resume":           doctor.Resume,
		})
	}
}
