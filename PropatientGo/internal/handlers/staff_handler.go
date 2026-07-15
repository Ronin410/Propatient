package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const staffInviteValidity = 72 * time.Hour

func generateInviteToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// ListStaff devuelve el personal vinculado al consultorio del doctor
// autenticado. Ruta protegida con RequireDoctorRole.
func ListStaff(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var staff []models.Staff
		if err := db.Where("doctor_id = ?", doctorID).Order("created_at ASC").Find(&staff).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener el personal"})
			return
		}

		c.JSON(http.StatusOK, staff)
	}
}

type inviteStaffRequest struct {
	FullName string `json:"fullName" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

// InviteStaff crea el registro de personal (sin contraseña todavía) y le
// manda un correo con un link para que la propia persona la establezca.
// Ruta protegida con RequireDoctorRole.
func InviteStaff(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var req inviteStaffRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))

		var existing models.Staff
		err := db.Where("LOWER(email) = ?", email).First(&existing).Error
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Ya existe una cuenta de personal con ese correo"})
			return
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al validar el correo"})
			return
		}

		token, err := generateInviteToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar la invitación"})
			return
		}
		expires := time.Now().UTC().Add(staffInviteValidity)

		staff := models.Staff{
			DoctorID:             doctorID,
			FullName:             req.FullName,
			Email:                email,
			Active:               true,
			PasswordSet:          false,
			InviteToken:          token,
			InviteTokenExpiresAt: &expires,
		}
		if err := db.Create(&staff).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo crear la cuenta de personal"})
			return
		}

		var doctor models.Doctor
		db.Select("full_name").First(&doctor, doctorID)

		inviteURL := fmt.Sprintf("%s/personal/invitacion/%s", frontendRedirectBase(), token)
		subject := "Te invitaron a ProPatient"
		body := fmt.Sprintf(`
			<html>
			<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
				<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 10px;">
					<h2 style="color: #002d42; text-align: center;">¡Hola, %s!</h2>
					<p>%s te invitó a acceder como personal del consultorio en <strong>ProPatient</strong>. Podrás gestionar la agenda y los pacientes, sin acceso al historial clínico ni a la configuración del doctor.</p>
					<p style="text-align: center; margin: 28px 0;">
						<a href="%s" style="background-color: #005073; color: #ffffff; padding: 12px 24px; border-radius: 6px; text-decoration: none; font-weight: 600;">Crear mi contraseña</a>
					</p>
					<p style="font-size: 12px; color: #888;">Este link vence en 72 horas. Si no esperabas esta invitación, puedes ignorar este correo.</p>
				</div>
			</body>
			</html>
		`, req.FullName, doctor.FullName, inviteURL)

		if err := auth.SendEmail(email, subject, body); err != nil {
			// El registro ya se creó; el doctor puede reenviar la invitación
			// más adelante si el correo falla (SMTP no configurado, etc.).
			c.JSON(http.StatusCreated, gin.H{
				"staff":   staff,
				"warning": "La cuenta se creó pero no se pudo enviar el correo de invitación.",
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"staff": staff})
	}
}

// ToggleStaffActive activa o desactiva el acceso de una cuenta de personal
// sin borrarla (para poder reactivarla después). Ruta protegida con
// RequireDoctorRole.
func ToggleStaffActive(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var req struct {
			Active bool `json:"active"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result := db.Model(&models.Staff{}).
			Where("id = ? AND doctor_id = ?", id, doctorID).
			Update("active", req.Active)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar el acceso"})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cuenta de personal no encontrada"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"active": req.Active})
	}
}

// DeleteStaff revoca permanentemente el acceso de una cuenta de personal.
// Ruta protegida con RequireDoctorRole.
func DeleteStaff(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		result := db.Where("id = ? AND doctor_id = ?", id, doctorID).Delete(&models.Staff{})
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo eliminar la cuenta de personal"})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cuenta de personal no encontrada"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Cuenta de personal eliminada"})
	}
}

// GetStaffInvite valida un token de invitación (público, sin JWT) y
// devuelve los datos básicos para que el frontend muestre "Hola, <nombre>"
// antes de pedir la contraseña.
func GetStaffInvite(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var staff models.Staff
		err := db.Where("invite_token = ? AND password_set = ?", token, false).First(&staff).Error
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invitación no válida"})
			return
		}
		if staff.InviteTokenExpiresAt == nil || staff.InviteTokenExpiresAt.Before(time.Now().UTC()) {
			c.JSON(http.StatusGone, gin.H{"error": "Esta invitación ya venció. Pide al doctor que te invite de nuevo."})
			return
		}

		var doctor models.Doctor
		db.Select("full_name").First(&doctor, staff.DoctorID)

		c.JSON(http.StatusOK, gin.H{
			"fullName":   staff.FullName,
			"email":      staff.Email,
			"doctorName": doctor.FullName,
		})
	}
}

// AcceptStaffInvite establece la contraseña definitiva de la cuenta de
// personal y consume el token de invitación. Público (sin JWT).
func AcceptStaffInvite(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var req struct {
			Password string `json:"password" binding:"required,min=6"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var staff models.Staff
		err := db.Where("invite_token = ? AND password_set = ?", token, false).First(&staff).Error
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invitación no válida"})
			return
		}
		if staff.InviteTokenExpiresAt == nil || staff.InviteTokenExpiresAt.Before(time.Now().UTC()) {
			c.JSON(http.StatusGone, gin.H{"error": "Esta invitación ya venció. Pide al doctor que te invite de nuevo."})
			return
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo procesar la contraseña"})
			return
		}

		err = db.Model(&staff).Updates(map[string]interface{}{
			"password_hash":           string(hashed),
			"password_set":            true,
			"invite_token":            "",
			"invite_token_expires_at": nil,
		}).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo activar la cuenta"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Contraseña creada. Ya puedes iniciar sesión."})
	}
}

type staffLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// StaffLoginHandler autentica una cuenta de personal por correo/contraseña
// y devuelve un JWT con doctorID = el doctor dueño del consultorio (ver
// GenerateStaffToken). Público.
func StaffLoginHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req staffLoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
			return
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))

		var staff models.Staff
		if err := db.Where("LOWER(email) = ?", email).First(&staff).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Correo o contraseña incorrectos"})
			return
		}
		if !staff.Active {
			c.JSON(http.StatusForbidden, gin.H{"error": "Esta cuenta de personal fue desactivada. Contacta al doctor."})
			return
		}
		if !staff.PasswordSet {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Aún no has creado tu contraseña. Revisa tu correo de invitación."})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(staff.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Correo o contraseña incorrectos"})
			return
		}

		var doctor models.Doctor
		db.Select("full_name").First(&doctor, staff.DoctorID)

		token, err := auth.GenerateStaffToken(staff.DoctorID, staff.ID, staff.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar la sesión"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token":      token,
			"fullName":   staff.FullName,
			"doctorName": doctor.FullName,
			"role":       "STAFF",
		})
	}
}
