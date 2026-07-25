package handlers

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"propatient-api/internal/models"
	"propatient-api/internal/storage"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const uploadLinkValidity = 30 * 24 * time.Hour

// GetAppointmentUploadLink genera (o reutiliza, mientras no haya vencido)
// el link público de "sube tus documentos antes de la cita" para una cita
// del doctor autenticado — el QR de ConsultationManager apunta aquí. A
// diferencia de exponer el ID crudo de la cita (como hacía el frontend
// antes de esto, sin backend real detrás), el token es opaco e
// impredecible: quien lo intercepte solo puede subir documentos a ESA
// cita, no adivinar/enumerar otras. Solo doctor (ver router.go), igual que
// el resto de la gestión de documentos de la cita.
func GetAppointmentUploadLink(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		appointmentID := c.Param("id")

		var appointment models.Appointment
		if err := db.Where("id = ? AND doctor_id = ?", appointmentID, doctorID).First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		if _, err := ensureAppointmentUploadToken(db, &appointment); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el link"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"uploadUrl": fmt.Sprintf("%s/public-upload/%s", frontendRedirectBase(), appointment.UploadToken),
		})
	}
}

// ensureAppointmentUploadToken genera (o reutiliza, mientras no haya
// vencido) el token del link público de "sube tus documentos antes de la
// cita" — compartido entre GetAppointmentUploadLink (el doctor lo copia/
// comparte a mano desde ConsultationManager) y CreateAppointment (se manda
// solo por WhatsApp en cuanto el doctor agenda, ver
// sendDoctorBookedAppointmentWhatsApp). Muta appointment.UploadToken in situ.
func ensureAppointmentUploadToken(db *gorm.DB, appointment *models.Appointment) (string, error) {
	if appointment.UploadToken != "" && appointment.UploadTokenExpiresAt != nil && appointment.UploadTokenExpiresAt.After(time.Now().UTC()) {
		return appointment.UploadToken, nil
	}

	token, err := generateInviteToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().UTC().Add(uploadLinkValidity)
	if err := db.Model(appointment).Updates(map[string]interface{}{
		"upload_token":            token,
		"upload_token_expires_at": expires,
	}).Error; err != nil {
		return "", err
	}
	appointment.UploadToken = token
	appointment.UploadTokenExpiresAt = &expires
	return token, nil
}

// findAppointmentByUploadToken resuelve una cita por su token de subida
// pública, rechazando tokens inventados o ya vencidos.
func findAppointmentByUploadToken(db *gorm.DB, token string) (models.Appointment, error) {
	var appointment models.Appointment
	if token == "" {
		return appointment, gorm.ErrRecordNotFound
	}
	err := db.Preload("Patient").
		Where("upload_token = ? AND upload_token_expires_at > ?", token, time.Now().UTC()).
		First(&appointment).Error
	return appointment, err
}

type publicUploadInfo struct {
	PatientName         string `json:"patientName"`
	DoctorName          string `json:"doctorName"`
	AppointmentDateTime string `json:"appointmentDateTime"`
	DocumentCount       int    `json:"documentCount"`
}

// GetPublicUploadInfo valida el token del link/QR (público, sin JWT) y
// devuelve los datos básicos para el saludo — mismo patrón que
// GetReviewInvite/GetStaffInvite. Nunca expone datos clínicos, solo lo
// necesario para confirmarle al paciente que está en el lugar correcto.
func GetPublicUploadInfo(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appointment, err := findAppointmentByUploadToken(db, c.Param("token"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Este link no es válido o ya venció."})
			return
		}

		var doctor models.Doctor
		db.Select("full_name").First(&doctor, appointment.DoctorID)

		var documentCount int64
		db.Model(&models.MedicalDocument{}).Where("appointment_id = ?", appointment.ID).Count(&documentCount)

		patientName := ""
		if appointment.Patient != nil {
			patientName = appointment.Patient.FirstName
		}

		c.JSON(http.StatusOK, publicUploadInfo{
			PatientName:         patientName,
			DoctorName:          doctor.FullName,
			AppointmentDateTime: appointment.AppointmentDateTime.Format(time.RFC3339),
			DocumentCount:       int(documentCount),
		})
	}
}

// PublicUploadDocuments recibe los archivos que el paciente sube desde su
// celular al escanear el QR (público, sin JWT) — mismo procesamiento que
// UploadDocuments (validación de tipo/tamaño, nombre de archivo propio
// para evitar path traversal), pero resuelve la cita por el token de
// subida en vez de doctorID+propiedad.
func PublicUploadDocuments(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		appointment, err := findAppointmentByUploadToken(db, c.Param("token"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Este link no es válido o ya venció."})
			return
		}

		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error al procesar los archivos"})
			return
		}

		files := form.File["files"]
		saved := 0
		for _, fileHeader := range files {
			if err := storage.ValidateUploadedFile(fileHeader, storage.UploadKindMedicalDocument); err != nil {
				log.Printf("⚠️ Documento %s rechazado (subida pública): %v", fileHeader.Filename, err)
				continue
			}

			ext := filepath.Ext(filepath.Base(fileHeader.Filename))
			key := fmt.Sprintf("documents/%d_%d%s", appointment.ID, time.Now().UnixNano(), ext)

			storedRef, err := storageClient.Save(c.Request.Context(), key, fileHeader)
			if err != nil {
				log.Printf("⚠️ No se pudo guardar el documento público %s: %v", fileHeader.Filename, err)
				continue
			}

			doc := models.MedicalDocument{
				FileName:      fileHeader.Filename,
				FileType:      fileHeader.Header.Get("Content-Type"),
				FilePath:      storedRef,
				AppointmentID: appointment.ID,
			}
			if err := db.Create(&doc).Error; err != nil {
				storageClient.Delete(c.Request.Context(), storedRef)
				continue
			}
			saved++
		}

		c.JSON(http.StatusOK, gin.H{"saved": saved})
	}
}
