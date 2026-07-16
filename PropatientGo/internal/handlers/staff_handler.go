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

// staffListItem es la forma que ve el doctor al listar su personal:
// campos propios de Staff + el estado del vínculo con ESTE consultorio
// (Active viene de DoctorStaff, ya no es un campo global de Staff, porque
// la misma persona puede estar activa en un consultorio e inactiva en
// otro).
type staffListItem struct {
	ID          uint   `json:"id"`
	FullName    string `json:"fullName"`
	Email       string `json:"email"`
	Active      bool   `json:"active"`
	PasswordSet bool   `json:"passwordSet"`
	DoctorID    uint   `json:"doctorId"`
}

// ListStaff devuelve el personal vinculado al consultorio del doctor
// autenticado. Ruta protegida con RequireDoctorRole.
func ListStaff(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var items []staffListItem
		err := db.Table("staffs").
			Select("staffs.id, staffs.full_name, staffs.email, staffs.password_set, doctor_staff.active, doctor_staff.doctor_id").
			Joins("JOIN doctor_staff ON doctor_staff.staff_id = staffs.id").
			Where("doctor_staff.doctor_id = ?", doctorID).
			Order("staffs.created_at ASC").
			Scan(&items).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener el personal"})
			return
		}

		c.JSON(http.StatusOK, items)
	}
}

type inviteStaffRequest struct {
	FullName string `json:"fullName" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

// InviteStaff vincula a una persona con el consultorio del doctor
// autenticado. Ruta protegida con RequireDoctorRole.
//
// Tres casos:
//  1. Correo nunca visto: se crea la cuenta de Staff (sin contraseña
//     todavía) y se manda el correo de invitación de siempre, para que la
//     propia persona cree su contraseña.
//  2. El correo ya tiene cuenta de Staff (invitada por CUALQUIER doctor,
//     no solo este) pero todavía no tiene vínculo con ESTE consultorio: se
//     crea el vínculo (DoctorStaff) nada más — la persona ya tiene
//     contraseña, no hace falta que haga nada más, solo se le avisa por
//     correo. Este es el caso de la clínica: la misma recepcionista queda
//     vinculada a varios doctores con una sola cuenta.
//  3. El correo ya está vinculado a ESTE consultorio: 409 si el vínculo
//     sigue activo, o se reactiva si estaba desactivado.
func InviteStaff(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var req inviteStaffRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))

		var doctor models.Doctor
		db.Select("full_name").First(&doctor, doctorID)

		var existing models.Staff
		err := db.Where("LOWER(email) = ?", email).First(&existing).Error

		if err == nil {
			linkExistingStaffToDoctor(c, db, doctorID, doctor.FullName, existing)
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
			FullName:             req.FullName,
			Email:                email,
			PasswordSet:          false,
			InviteToken:          token,
			InviteTokenExpiresAt: &expires,
		}
		if err := db.Create(&staff).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo crear la cuenta de personal"})
			return
		}
		if err := db.Create(&models.DoctorStaff{DoctorID: doctorID, StaffID: staff.ID, Active: true}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo vincular al personal con tu consultorio"})
			return
		}

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

// linkExistingStaffToDoctor maneja el caso de InviteStaff donde el correo
// ya tiene una cuenta de Staff (de este doctor o de otro).
func linkExistingStaffToDoctor(c *gin.Context, db *gorm.DB, doctorID uint, doctorName string, staff models.Staff) {
	var link models.DoctorStaff
	linkErr := db.Where("doctor_id = ? AND staff_id = ?", doctorID, staff.ID).First(&link).Error

	if linkErr == nil {
		if link.Active {
			c.JSON(http.StatusConflict, gin.H{"error": "Esta persona ya tiene acceso a tu consultorio"})
			return
		}
		if err := db.Model(&link).Update("active", true).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo reactivar el acceso"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"staff": staff, "message": "Se reactivó el acceso de esta persona a tu consultorio."})
		return
	}
	if !errors.Is(linkErr, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al vincular al personal"})
		return
	}

	if err := db.Create(&models.DoctorStaff{DoctorID: doctorID, StaffID: staff.ID, Active: true}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo vincular al personal con tu consultorio"})
		return
	}

	// Best-effort: esta persona ya tiene cuenta y contraseña, no hace
	// falta que haga nada — solo se le avisa que ahora también puede
	// entrar a este otro consultorio.
	subject := "Ahora tienes acceso a otro consultorio en ProPatient"
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 10px;">
				<h2 style="color: #002d42; text-align: center;">¡Hola, %s!</h2>
				<p>%s también te dio acceso como personal de su consultorio en <strong>ProPatient</strong>, con la misma cuenta que ya tienes. La próxima vez que inicies sesión vas a poder elegir con cuál consultorio entrar.</p>
			</div>
		</body>
		</html>
	`, staff.FullName, doctorName)
	go func(toEmail, subj, htmlBody string) {
		defer func() { recover() }()
		auth.SendEmail(toEmail, subj, htmlBody)
	}(staff.Email, subject, body)

	c.JSON(http.StatusCreated, gin.H{"staff": staff})
}

// ToggleStaffActive activa o desactiva el acceso de una cuenta de personal
// A ESTE consultorio, sin borrarla (para poder reactivarla después) y sin
// afectar su acceso a otros consultorios donde también trabaje. Ruta
// protegida con RequireDoctorRole.
func ToggleStaffActive(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		staffID := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var req struct {
			Active bool `json:"active"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result := db.Model(&models.DoctorStaff{}).
			Where("staff_id = ? AND doctor_id = ?", staffID, doctorID).
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

// DeleteStaff revoca el acceso de una cuenta de personal a ESTE
// consultorio (borra el vínculo, no la cuenta de Staff compartida — si
// esta persona también trabaja para otro doctor, ese acceso no se toca).
// Ruta protegida con RequireDoctorRole.
func DeleteStaff(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		staffID := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		result := db.Where("staff_id = ? AND doctor_id = ?", staffID, doctorID).Delete(&models.DoctorStaff{})
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

		var link models.DoctorStaff
		db.Where("staff_id = ?", staff.ID).Order("created_at ASC").First(&link)

		var doctor models.Doctor
		db.Select("full_name").First(&doctor, link.DoctorID)

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

// staffDoctorOption identifica un consultorio al que una cuenta de
// personal tiene acceso activo.
type staffDoctorOption struct {
	DoctorID   uint   `json:"doctorId"`
	DoctorName string `json:"doctorName"`
}

// activeDoctorsForStaff devuelve los consultorios donde este personal
// tiene acceso activo hoy, ordenados por nombre.
func activeDoctorsForStaff(db *gorm.DB, staffID uint) ([]staffDoctorOption, error) {
	var doctors []staffDoctorOption
	err := db.Table("doctor_staff").
		Select("doctor_staff.doctor_id AS doctor_id, doctors.full_name AS doctor_name").
		Joins("JOIN doctors ON doctors.id = doctor_staff.doctor_id").
		Where("doctor_staff.staff_id = ? AND doctor_staff.active = ?", staffID, true).
		Order("doctors.full_name ASC").
		Scan(&doctors).Error
	return doctors, err
}

// StaffLoginHandler autentica una cuenta de personal por correo/contraseña.
// Si tiene acceso activo a un solo consultorio, devuelve el JWT de sesión
// de una vez (mismo comportamiento de siempre). Si tiene acceso a varios
// (clínica con personal compartido), en vez de asumir uno devuelve un
// token de selección de corta vida y la lista de consultorios, para que
// el frontend pida elegir uno (ver SelectStaffDoctorHandler). Público.
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
		if !staff.PasswordSet {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Aún no has creado tu contraseña. Revisa tu correo de invitación."})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(staff.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Correo o contraseña incorrectos"})
			return
		}

		doctors, err := activeDoctorsForStaff(db, staff.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al validar el acceso"})
			return
		}
		if len(doctors) == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "No tienes acceso activo a ningún consultorio. Contacta al doctor."})
			return
		}

		if len(doctors) == 1 {
			token, err := auth.GenerateStaffToken(doctors[0].DoctorID, staff.ID, staff.Email)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar la sesión"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"token":      token,
				"fullName":   staff.FullName,
				"doctorName": doctors[0].DoctorName,
				"role":       "STAFF",
			})
			return
		}

		selectionToken, err := auth.GenerateStaffSelectionToken(staff.ID, staff.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar la sesión"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"needsDoctorSelection": true,
			"selectionToken":       selectionToken,
			"doctors":              doctors,
		})
	}
}

type selectStaffDoctorRequest struct {
	SelectionToken string `json:"selectionToken" binding:"required"`
	DoctorID       uint   `json:"doctorId" binding:"required"`
}

// SelectStaffDoctorHandler es el segundo paso del login de personal
// cuando tiene acceso activo a más de un consultorio: recibe el token de
// selección emitido por StaffLoginHandler (prueba que la contraseña ya se
// validó) y el doctorId elegido, y emite el JWT real de sesión para ESE
// consultorio. Público — usa su propio token de corta vida, no un JWT de
// sesión normal.
func SelectStaffDoctorHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req selectStaffDoctorRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		staffID, err := auth.ValidateStaffSelectionToken(req.SelectionToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesión de selección inválida o vencida. Inicia sesión de nuevo."})
			return
		}

		var staff models.Staff
		if err := db.First(&staff, staffID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cuenta de personal no encontrada"})
			return
		}

		var link models.DoctorStaff
		err = db.Where("staff_id = ? AND doctor_id = ? AND active = ?", staffID, req.DoctorID, true).First(&link).Error
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "No tienes acceso activo a ese consultorio."})
			return
		}

		var doctor models.Doctor
		db.Select("full_name").First(&doctor, req.DoctorID)

		token, err := auth.GenerateStaffToken(req.DoctorID, staff.ID, staff.Email)
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

const staffPasswordResetValidity = 1 * time.Hour

// requestStaffPasswordResetGenericResponse es la MISMA respuesta sin
// importar si el correo existe, tiene acceso activo o ya tiene contraseña
// — responder distinto en cada caso permitiría a cualquiera usar este
// endpoint para averiguar qué correos están registrados como personal.
const requestStaffPasswordResetGenericResponse = "Si el correo está registrado, te enviamos instrucciones para restablecer tu contraseña."

type requestStaffPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// RequestStaffPasswordReset genera un token de reseteo y lo manda por
// correo si el email corresponde a una cuenta de personal con contraseña
// ya creada y con acceso activo a al menos un consultorio. Público (sin
// JWT). Siempre responde 200 con el mismo mensaje genérico, exista o no
// la cuenta.
func RequestStaffPasswordReset(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req requestStaffPasswordResetRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))

		var staff models.Staff
		err := db.Where("LOWER(email) = ?", email).First(&staff).Error
		if err != nil || !staff.PasswordSet {
			// No revelamos cuál de estas condiciones falló.
			c.JSON(http.StatusOK, gin.H{"message": requestStaffPasswordResetGenericResponse})
			return
		}
		var activeLinks int64
		db.Model(&models.DoctorStaff{}).Where("staff_id = ? AND active = ?", staff.ID, true).Count(&activeLinks)
		if activeLinks == 0 {
			c.JSON(http.StatusOK, gin.H{"message": requestStaffPasswordResetGenericResponse})
			return
		}

		token, err := generateInviteToken()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"message": requestStaffPasswordResetGenericResponse})
			return
		}
		expires := time.Now().UTC().Add(staffPasswordResetValidity)
		db.Model(&staff).Updates(map[string]interface{}{
			"password_reset_token":            token,
			"password_reset_token_expires_at": expires,
		})

		resetURL := fmt.Sprintf("%s/personal/restablecer/%s", frontendRedirectBase(), token)
		subject := "ProPatient - Restablece tu contraseña"
		body := fmt.Sprintf(`
			<html>
			<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
				<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 10px;">
					<h2 style="color: #002d42; text-align: center;">Restablece tu contraseña</h2>
					<p>Recibimos una solicitud para restablecer la contraseña de tu cuenta de personal en <strong>ProPatient</strong>.</p>
					<p style="text-align: center; margin: 28px 0;">
						<a href="%s" style="background-color: #005073; color: #ffffff; padding: 12px 24px; border-radius: 6px; text-decoration: none; font-weight: 600;">Restablecer contraseña</a>
					</p>
					<p style="font-size: 12px; color: #888;">Este link vence en 1 hora. Si no lo solicitaste, puedes ignorar este correo — tu contraseña actual sigue funcionando.</p>
				</div>
			</body>
			</html>
		`, resetURL)

		// Best-effort en segundo plano: la respuesta ya es genérica sin
		// importar si el correo se manda o no, así que no hay razón para
		// bloquear al usuario esperando al proveedor de correo.
		go func(toEmail, htmlBody string) {
			defer func() { recover() }()
			if err := auth.SendEmail(toEmail, subject, htmlBody); err != nil {
				fmt.Printf("❌ Fallo al enviar correo de reseteo de contraseña a [%s]: %v\n", toEmail, err)
			}
		}(staff.Email, body)

		c.JSON(http.StatusOK, gin.H{"message": requestStaffPasswordResetGenericResponse})
	}
}

type resetStaffPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6"`
}

// ResetStaffPassword consume el token de reseteo y establece la nueva
// contraseña. Público (sin JWT).
func ResetStaffPassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var req resetStaffPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var staff models.Staff
		err := db.Where("password_reset_token = ? AND password_reset_token != ?", token, "").First(&staff).Error
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Este link de reseteo no es válido."})
			return
		}
		if staff.PasswordResetTokenExpiresAt == nil || staff.PasswordResetTokenExpiresAt.Before(time.Now().UTC()) {
			c.JSON(http.StatusGone, gin.H{"error": "Este link de reseteo ya venció. Solicita uno nuevo."})
			return
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo procesar la contraseña"})
			return
		}

		err = db.Model(&staff).Updates(map[string]interface{}{
			"password_hash":                   string(hashed),
			"password_reset_token":            "",
			"password_reset_token_expires_at": nil,
		}).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo restablecer la contraseña"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Contraseña actualizada. Ya puedes iniciar sesión."})
	}
}
