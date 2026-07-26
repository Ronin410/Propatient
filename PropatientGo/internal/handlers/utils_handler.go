package handlers

import (
	"net/http"
	"propatient-api/internal/models"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

// SearchCie10Codes busca en el catálogo CIE-10 (ver
// database.SeedCie10Catalog) por código (prefijo) o por nombre
// (subcadena) — usado por el buscador de diagnóstico en la consulta.
// Exige al menos 2 caracteres para no regresar el catálogo casi completo
// en la primera tecla.
func SearchCie10Codes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := strings.TrimSpace(c.Query("q"))
		if len(query) < 2 {
			c.JSON(http.StatusOK, []models.Cie10Code{})
			return
		}

		var results []models.Cie10Code
		term := "%" + query + "%"
		err := db.Where("code ILIKE ? OR name ILIKE ?", query+"%", term).
			Order("code ASC").
			Limit(20).
			Find(&results).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al buscar en el catálogo CIE-10"})
			return
		}
		c.JSON(http.StatusOK, results)
	}
}
