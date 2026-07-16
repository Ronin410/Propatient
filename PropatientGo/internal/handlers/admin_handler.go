package handlers

import (
	"fmt"
	"net/http"

	"propatient-api/internal/auth"
	"propatient-api/internal/models"
	"propatient-api/internal/storage"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SuperAdminLoginHandler autentica una cuenta interna de ProPatient (ver
// models.SuperAdmin), totalmente separada de Doctor/Staff. Mismo patrón
// que auth.LoginHandler, pero emite un GenerateSuperAdminToken.
func SuperAdminLoginHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
			return
		}

		var admin models.SuperAdmin
		if err := db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario o contraseña incorrectos"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario o contraseña incorrectos"})
			return
		}

		token, err := auth.GenerateSuperAdminToken(admin.ID, admin.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar la sesión"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token, "username": admin.Username})
	}
}

// pendingDoctorResponse expone solo lo que el administrador necesita para
// revisar una cédula: nunca el hash de contraseña ni datos clínicos (este
// endpoint no tiene ninguna relación con pacientes).
type pendingDoctorResponse struct {
	ID              uint   `json:"id"`
	FullName        string `json:"fullName"`
	Email           string `json:"email"`
	Username        string `json:"username"`
	LicenseNumber   string `json:"licenseNumber"`
	RFC             string `json:"rfc"`
	CURP            string `json:"curp"`
	University      string `json:"university"`
	IneDocumentURL  string `json:"ineDocumentUrl"`
	CedulaValidated string `json:"cedulaValidated"`
}

// ListPendingDoctors devuelve los doctores con cédula "CAPTURADA": ya
// terminaron el onboarding y están esperando revisión manual.
func ListPendingDoctors(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var doctors []models.Doctor
		if err := db.Where("cedula_validated = ?", "CAPTURADA").Order("updated_at asc").Find(&doctors).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al consultar doctores pendientes"})
			return
		}

		response := make([]pendingDoctorResponse, 0, len(doctors))
		for _, d := range doctors {
			ineURL := ""
			if d.IneDocumentPath != "" {
				if url, err := storageClient.URL(c.Request.Context(), d.IneDocumentPath); err == nil {
					ineURL = url
				}
			}
			response = append(response, pendingDoctorResponse{
				ID:              d.ID,
				FullName:        d.FullName,
				Email:           d.Email,
				Username:        d.Username,
				LicenseNumber:   d.LicenseNumber,
				RFC:             d.RFC,
				CURP:            d.CURP,
				University:      d.University,
				IneDocumentURL:  ineURL,
				CedulaValidated: d.CedulaValidated,
			})
		}

		c.JSON(http.StatusOK, response)
	}
}

// ApproveDoctorCedula marca a un doctor como VALIDADA y le avisa por
// correo. Solo tiene efecto si estaba en CAPTURADA — evita "aprobar" a un
// doctor que ni siquiera ha terminado su onboarding.
func ApproveDoctorCedula(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var doctor models.Doctor
		if err := db.First(&doctor, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.CedulaValidated != "CAPTURADA" {
			c.JSON(http.StatusConflict, gin.H{"error": "Este doctor no tiene una cédula pendiente de revisión"})
			return
		}

		if err := db.Model(&doctor).Update("cedula_validated", "VALIDADA").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al aprobar la cédula"})
			return
		}

		go func(email, name string) {
			defer func() { recover() }()
			if email == "" {
				return
			}
			if err := auth.SendCedulaApprovedEmail(email, name); err != nil {
				fmt.Printf("❌ Fallo al enviar correo de aprobación de cédula a [%s]: %v\n", email, err)
			}
		}(doctor.Email, doctor.FullName)

		c.JSON(http.StatusOK, gin.H{"message": "Cédula aprobada"})
	}
}

// RejectDoctorCedula regresa al doctor a PENDIENTE (puede volver a subir su
// documentación desde ValidateLicense.tsx) y le avisa por correo con un
// motivo opcional.
func RejectDoctorCedula(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req struct {
			Reason string `json:"reason"`
		}
		// El body es opcional: si no viene o no es JSON válido, se rechaza
		// sin motivo en vez de fallar la solicitud completa.
		_ = c.ShouldBindJSON(&req)

		var doctor models.Doctor
		if err := db.First(&doctor, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.CedulaValidated != "CAPTURADA" {
			c.JSON(http.StatusConflict, gin.H{"error": "Este doctor no tiene una cédula pendiente de revisión"})
			return
		}

		if err := db.Model(&doctor).Update("cedula_validated", "PENDIENTE").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al rechazar la cédula"})
			return
		}

		go func(email, name, reason string) {
			defer func() { recover() }()
			if email == "" {
				return
			}
			if err := auth.SendCedulaRejectedEmail(email, name, reason); err != nil {
				fmt.Printf("❌ Fallo al enviar correo de rechazo de cédula a [%s]: %v\n", email, err)
			}
		}(doctor.Email, doctor.FullName, req.Reason)

		c.JSON(http.StatusOK, gin.H{"message": "Cédula rechazada"})
	}
}
