package handlers

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"propatient-api/internal/geocoding"
	"propatient-api/internal/models"
	"propatient-api/internal/storage"
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

func GetCurrentDoctor(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		presignDoctorFiles(c.Request.Context(), storageClient, &doctor)

		// GORM ya tiene json:"-" en PasswordHash y en el refresh token de
		// Google Calendar en el modelo, por lo que nunca se envían.
		c.JSON(http.StatusOK, doctorWithCalendarStatus{
			Doctor:                  doctor,
			GoogleCalendarConnected: doctor.GoogleCalendarRefreshToken != "",
		})
	}
}

func UpdateCurrentDoctor(db *gorm.DB, storageClient storage.Client, geoClient geocoding.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		previousAddress := doctor.Address

		// 1. Procesar y guardar el AVATAR (Foto de perfil) si viene en la petición
		avatarFile, err := c.FormFile("avatar")
		if err == nil && avatarFile != nil {
			ext := filepath.Ext(avatarFile.Filename)
			key := fmt.Sprintf("profiles/doc_%d_avatar_%d%s", doctorID, time.Now().Unix(), ext)

			storedRef, err := storageClient.Save(c.Request.Context(), key, avatarFile)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar la foto de perfil"})
				return
			}
			doctor.AvatarUrl = storedRef
		}

		// 2. Procesar y guardar el LOGO de la clínica si viene en la petición
		logoFile, err := c.FormFile("logo")
		if err == nil && logoFile != nil {
			ext := filepath.Ext(logoFile.Filename)
			key := fmt.Sprintf("profiles/doc_%d_logo_%d%s", doctorID, time.Now().Unix(), ext)

			storedRef, err := storageClient.Save(c.Request.Context(), key, logoFile)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar el logo de la clínica"})
				return
			}
			doctor.LogoUrl = storedRef
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

		// Directorio público (landing page): opt-in explícito del doctor.
		// El slug se genera una sola vez (nunca cambia, para no romper
		// enlaces ya compartidos) y la geocodificación solo se repite si
		// la dirección cambió, para no golpear la API de Nominatim en
		// cada guardado del perfil.
		doctor.PublicListed = c.PostForm("publicListed") == "true"
		doctor.PublicBio = c.PostForm("publicBio")
		if doctor.PublicListed {
			if doctor.PublicSlug == "" {
				doctor.PublicSlug = generateDoctorSlug(doctor.FullName, doctor.ID)
			}
			if doctor.Address != "" && (doctor.Address != previousAddress || doctor.Latitude == nil) {
				if coords, err := geoClient.Geocode(c.Request.Context(), doctor.Address); err != nil {
					log.Printf("⚠️ No se pudo geocodificar la dirección del doctor %d: %v", doctorID, err)
				} else if coords != nil {
					doctor.Latitude = &coords.Latitude
					doctor.Longitude = &coords.Longitude
				}
			}
		}

		// 5. Persistir los cambios en la Base de Datos
		if err := db.Save(&doctor).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar el perfil"})
			return
		}

		presignDoctorFiles(c.Request.Context(), storageClient, &doctor)

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
			"publicListed":     doctor.PublicListed,
			"publicBio":        doctor.PublicBio,
			"publicSlug":       doctor.PublicSlug,
		})
	}
}
