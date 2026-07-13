package handlers

import (
	"net/http"
	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type UpdateTemplateInput struct {
	Fields datatypes.JSON `json:"fields" binding:\"required\"`
}

// GET /api/doctor/template
func GetDoctorTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// NOTA: Aquí deberías obtener el DoctorID real desde el token JWT o sesión. 
		// Por ahora simularemos el ID 1 para desarrollo.
		doctorID := uint(1) 

		var template models.DoctorTemplate
		err := db.Where("doctor_id = ?", doctorID).First(&template).Error
		
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Si no existe, devolvemos un objeto vacío o valores por defecto para que el front no rompa
				c.JSON(http.StatusOK, gin.H{"doctorId": doctorID, "fields": []interface{}{}})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener la plantilla"})
			return
		}

		c.JSON(http.StatusOK, template)
	}
}

// POST /api/doctor/template
func SaveDoctorTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := uint(1) // Cambiar por la extracción real del JWT

		var input UpdateTemplateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var template models.DoctorTemplate
		// Buscamos si ya tiene una plantilla, si existe la actualiza, si no, la crea (Upsert)
		err := db.Where("doctor_id = ?", doctorID).First(&template).Error

		if err == gorm.ErrRecordNotFound {
			newTemplate := models.DoctorTemplate{
				DoctorID: doctorID,
				Fields:   input.Fields,
			}
			if err := db.Create(&newTemplate).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al crear la plantilla"})
				return
			}
			c.JSON(http.StatusCreated, newTemplate)
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al buscar la plantilla"})
			return
		}

		// Si ya existía, actualizamos los campos
		template.Fields = input.Fields
		if err := db.Save(&template).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar la plantilla"})
			return
		}

		c.JSON(http.StatusOK, template)
	}
}