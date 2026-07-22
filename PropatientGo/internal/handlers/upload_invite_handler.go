package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"propatient-api/internal/auth"
	"propatient-api/internal/models"
	"propatient-api/internal/storage"
	"propatient-api/internal/whatsapp"
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

		if appointment.UploadToken == "" || appointment.UploadTokenExpiresAt == nil || appointment.UploadTokenExpiresAt.Before(time.Now().UTC()) {
			token, err := generateInviteToken()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el link"})
				return
			}
			expires := time.Now().UTC().Add(uploadLinkValidity)
			if err := db.Model(&appointment).Updates(map[string]interface{}{
				"upload_token":            token,
				"upload_token_expires_at": expires,
			}).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar el link"})
				return
			}
			appointment.UploadToken = token
		}

		c.JSON(http.StatusOK, gin.H{
			"uploadUrl": fmt.Sprintf("%s/public-upload/%s", frontendRedirectBase(), appointment.UploadToken),
		})
	}
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
func PublicUploadDocuments(db *gorm.DB, storageClient storage.Client, waClient whatsapp.Client, waTemplates whatsapp.Templates) gin.HandlerFunc {
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

		// Aviso al doctor (correo + WhatsApp, mejor esfuerzo) de que un
		// paciente subió documentos antes de su cita — antes no se avisaba
		// nada, el doctor solo lo notaba si entraba a revisar la cita. En
		// segundo plano: ver el comentario largo en CreatePublicAppointment
		// sobre por qué esto no debe bloquear la respuesta ni usar el
		// contexto de la petición.
		if saved > 0 {
			var doctor models.Doctor
			if db.First(&doctor, appointment.DoctorID).Error == nil {
				patientName := "Un paciente"
				if appointment.Patient != nil {
					patientName = fmt.Sprintf("%s %s", appointment.Patient.FirstName, appointment.Patient.LastName)
				}
				go func(doctor models.Doctor, patientName string, appointment models.Appointment, saved int) {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("⚠️ Pánico recuperado al mandar el aviso de documentos subidos (cita %d): %v", appointment.ID, r)
						}
					}()
					sendDocumentUploadedWhatsApp(context.Background(), waClient, waTemplates, doctor, patientName, appointment, saved)
					sendDocumentUploadedEmail(doctor, patientName, appointment, saved)
				}(doctor, patientName, appointment, saved)
			}
		}

		c.JSON(http.StatusOK, gin.H{"saved": saved})
	}
}

// sendDocumentUploadedWhatsApp avisa al doctor, por WhatsApp, que un
// paciente subió documentos desde el link/QR antes de su cita. Mejor
// esfuerzo, mismo criterio que el resto de los avisos de esta app —
// siempre se manda por los dos canales (a diferencia de los avisos al
// paciente, este no tiene lógica de "no duplicar").
func sendDocumentUploadedWhatsApp(ctx context.Context, waClient whatsapp.Client, waTemplates whatsapp.Templates, doctor models.Doctor, patientName string, appointment models.Appointment, saved int) {
	if waClient == nil || doctor.Phone == "" {
		return
	}
	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)
	body := fmt.Sprintf(
		"%s subió %d documento(s) para su cita del %s. Revísalos en tu panel de ProPatient.",
		patientName, saved, when,
	)
	vars := map[string]string{"1": patientName, "2": fmt.Sprintf("%d", saved), "3": when}
	if err := whatsapp.SendWithFallback(ctx, waClient, doctor.Phone, waTemplates.DocumentUploaded, vars, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el WhatsApp de documentos subidos al doctor %d: %v", doctor.ID, err)
	}
}

// sendDocumentUploadedEmail es el equivalente por correo de
// sendDocumentUploadedWhatsApp.
func sendDocumentUploadedEmail(doctor models.Doctor, patientName string, appointment models.Appointment, saved int) {
	if doctor.Email == "" {
		return
	}
	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)
	subject := "Un paciente subió documentos para su cita"
	body := fmt.Sprintf(
		`<p><strong>%s</strong> subió <strong>%d</strong> documento(s) para su cita del <strong>%s</strong>.</p>
		<p>Revísalos desde tu panel de ProPatient.</p>
		<p>— ProPatient</p>`,
		patientName, saved, when,
	)
	if err := auth.SendEmail(doctor.Email, subject, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el correo de documentos subidos al doctor %d: %v", doctor.ID, err)
	}
}
