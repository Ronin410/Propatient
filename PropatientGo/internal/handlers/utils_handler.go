package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// GetSpecialties devuelve una lista estática de especialidades médicas
func GetSpecialties(c *gin.Context) {
	specialties := []string{
		"Medicina General",
		"Cardiología",
		"Pediatría",
		"Ginecología",
		"Dermatología",
		"Oftalmología",
		"Psiquiatría",
		"Traumatología",
		"Nutrición",
	}
	c.JSON(http.StatusOK, specialties)
}
