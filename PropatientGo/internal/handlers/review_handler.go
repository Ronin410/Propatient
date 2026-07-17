package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"propatient-api/internal/models"
	"propatient-api/internal/whatsapp"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const reviewInviteValidity = 30 * 24 * time.Hour

// createReviewInviteIfMissing crea la fila de Review (con token, sin
// calificación todavía) para una cita recién marcada COMPLETED. Devuelve
// (nil, nil) si ya existía una para esta cita — AppointmentID es único, así
// que guardar la cita completada varias veces (ej. editar notas después)
// nunca manda una segunda invitación a reseña.
func createReviewInviteIfMissing(db *gorm.DB, doctorID, patientID, appointmentID uint) (*models.Review, error) {
	var existing models.Review
	err := db.Where("appointment_id = ?", appointmentID).First(&existing).Error
	if err == nil {
		return nil, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	token, err := generateInviteToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(reviewInviteValidity)

	review := models.Review{
		DoctorID:       doctorID,
		PatientID:      patientID,
		AppointmentID:  appointmentID,
		Token:          token,
		TokenExpiresAt: &expires,
	}
	if err := db.Create(&review).Error; err != nil {
		return nil, err
	}
	return &review, nil
}

// sendReviewRequestWhatsApp avisa al paciente, por WhatsApp, que puede
// dejar una reseña de su consulta recién terminada. Mejor esfuerzo, igual
// que el resto de los avisos de WhatsApp de esta app.
func sendReviewRequestWhatsApp(ctx context.Context, waClient whatsapp.Client, doctor models.Doctor, patient models.Patient, token string) {
	if waClient == nil || patient.Phone == "" {
		return
	}
	link := fmt.Sprintf("%s/resena/%s", frontendRedirectBase(), token)
	body := fmt.Sprintf(
		"Hola %s, gracias por tu consulta con Dr(a). %s. ¿Nos regalas unos segundos para calificarla? %s — ProPatient",
		patient.FirstName, doctor.FullName, link,
	)
	if err := waClient.SendMessage(ctx, patient.Phone, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el WhatsApp de solicitud de reseña al paciente %d: %v", patient.ID, err)
	}
}

// reviewInviteInfo es lo que ve el paciente antes de calificar: solo lo
// necesario para el saludo, nunca datos clínicos.
type reviewInviteInfo struct {
	DoctorName   string `json:"doctorName"`
	PatientName  string `json:"patientName"`
	AlreadyRated bool   `json:"alreadyRated"`
}

// GetReviewInvite valida el token de una invitación a reseña (público, sin
// JWT) y devuelve los datos básicos para el saludo — mismo patrón que
// GetStaffInvite.
func GetReviewInvite(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var review models.Review
		if err := db.Where("token = ?", token).First(&review).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Este link de reseña no es válido."})
			return
		}
		if review.TokenExpiresAt == nil || review.TokenExpiresAt.Before(time.Now().UTC()) {
			c.JSON(http.StatusGone, gin.H{"error": "Este link de reseña ya venció."})
			return
		}

		var doctor models.Doctor
		var patient models.Patient
		db.Select("full_name").First(&doctor, review.DoctorID)
		db.Select("first_name").First(&patient, review.PatientID)

		c.JSON(http.StatusOK, reviewInviteInfo{
			DoctorName:   doctor.FullName,
			PatientName:  patient.FirstName,
			AlreadyRated: review.SubmittedAt != nil,
		})
	}
}

type submitReviewRequest struct {
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment"`
}

// SubmitReview consume el token y guarda la calificación/comentario
// (público, sin JWT). No se puede reenviar dos veces con el mismo token —
// una vez que SubmittedAt ya tiene fecha, el token queda consumido, mismo
// criterio que AcceptStaffInvite con InviteToken.
func SubmitReview(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var req submitReviewRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var review models.Review
		if err := db.Where("token = ?", token).First(&review).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Este link de reseña no es válido."})
			return
		}
		if review.TokenExpiresAt == nil || review.TokenExpiresAt.Before(time.Now().UTC()) {
			c.JSON(http.StatusGone, gin.H{"error": "Este link de reseña ya venció."})
			return
		}
		if review.SubmittedAt != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Ya habías enviado esta reseña."})
			return
		}

		now := time.Now().UTC()
		if err := db.Model(&review).Updates(map[string]interface{}{
			"rating":       req.Rating,
			"comment":      req.Comment,
			"submitted_at": now,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar tu reseña"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "¡Gracias por tu reseña!"})
	}
}

// ListReviews devuelve todas las reseñas ya ENVIADAS (no las invitaciones
// sin contestar) del consultorio del doctor autenticado, más recientes
// primero — solo doctor (ver router.go), es una herramienta de reputación/
// marketing, no algo que el personal necesite gestionar.
func ListReviews(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var reviews []struct {
			models.Review
			PatientFirstName string `json:"patientFirstName"`
			PatientLastName  string `json:"patientLastName"`
		}
		err := db.Table("reviews").
			Select("reviews.*, patients.first_name AS patient_first_name, patients.last_name AS patient_last_name").
			Joins("JOIN patients ON patients.id = reviews.patient_id").
			Where("reviews.doctor_id = ? AND reviews.submitted_at IS NOT NULL", doctorID).
			Order("reviews.submitted_at DESC").
			Scan(&reviews).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron obtener las reseñas"})
			return
		}

		c.JSON(http.StatusOK, reviews)
	}
}

// SetReviewApproval aprueba o retira una reseña del perfil público (solo
// doctor). No se puede aprobar una reseña que nunca se envió (Rating 0).
func SetReviewApproval(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		id := c.Param("id")

		var req struct {
			Approved bool `json:"approved"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result := db.Model(&models.Review{}).
			Where("id = ? AND doctor_id = ? AND submitted_at IS NOT NULL", id, doctorID).
			Update("approved", req.Approved)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar la reseña"})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Reseña no encontrada"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"approved": req.Approved})
	}
}
