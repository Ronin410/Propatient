package handlers

import (
	"net/http"
	"github.com/gin-gonic/gin"
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