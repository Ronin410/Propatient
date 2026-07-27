package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"propatient-api/internal/audit"
	"propatient-api/internal/auth"
	"propatient-api/internal/googlecalendar"
	"propatient-api/internal/models"
	"propatient-api/internal/storage"
	"propatient-api/internal/whatsapp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// followUpEventDuration es la duración por defecto que se le asigna a una
// cita al reflejarla en Google Calendar (el modelo de Appointment no guarda
// una duración explícita, solo la hora de inicio).
const googleCalendarEventDuration = 30 * time.Minute

// syncAppointmentToGoogleCalendar crea o actualiza (según si ya existe
// GoogleEventID) el evento espejo de una cita en el calendario del doctor.
// Es "best effort": si el doctor no tiene Google Calendar conectado, o si la
// llamada a Google falla, no se propaga el error — la cita en PROPatient ya
// se guardó y no debe fallar por un problema ajeno a la app.
func syncAppointmentToGoogleCalendar(ctx context.Context, db *gorm.DB, calClient googlecalendar.Client, appointment *models.Appointment, patient models.Patient) {
	if calClient == nil {
		return
	}

	var doctor models.Doctor
	if err := db.Select("google_calendar_refresh_token").First(&doctor, appointment.DoctorID).Error; err != nil {
		return
	}
	if doctor.GoogleCalendarRefreshToken == "" {
		return
	}

	ev := googlecalendar.EventInput{
		Summary:     fmt.Sprintf("Cita: %s %s", patient.FirstName, patient.LastName),
		Description: appointment.Reason,
		Start:       appointment.AppointmentDateTime,
		End:         appointment.AppointmentDateTime.Add(googleCalendarEventDuration),
	}

	if appointment.GoogleEventID == "" {
		eventID, err := calClient.CreateEvent(ctx, doctor.GoogleCalendarRefreshToken, ev)
		if err != nil {
			log.Printf("⚠️ No se pudo crear el evento de Google Calendar para la cita %d: %v", appointment.ID, err)
			return
		}
		db.Model(&models.Appointment{}).Where("id = ?", appointment.ID).Update("google_event_id", eventID)
		return
	}

	if err := calClient.UpdateEvent(ctx, doctor.GoogleCalendarRefreshToken, appointment.GoogleEventID, ev); err != nil {
		log.Printf("⚠️ No se pudo actualizar el evento de Google Calendar de la cita %d: %v", appointment.ID, err)
	}
}

// deleteAppointmentFromGoogleCalendar borra el evento espejo al cancelar una
// cita. También "best effort", igual que la sincronización de arriba.
func deleteAppointmentFromGoogleCalendar(ctx context.Context, db *gorm.DB, calClient googlecalendar.Client, appointment models.Appointment) {
	if calClient == nil || appointment.GoogleEventID == "" {
		return
	}

	var doctor models.Doctor
	if err := db.Select("google_calendar_refresh_token").First(&doctor, appointment.DoctorID).Error; err != nil {
		return
	}
	if doctor.GoogleCalendarRefreshToken == "" {
		return
	}

	if err := calClient.DeleteEvent(ctx, doctor.GoogleCalendarRefreshToken, appointment.GoogleEventID); err != nil {
		log.Printf("⚠️ No se pudo borrar el evento de Google Calendar de la cita %d: %v", appointment.ID, err)
	}
}

// CreateAppointmentRequest define la estructura para las solicitudes de creación de citas.
// Soporta la vinculación a un paciente existente (a través de PatientID) o la creación de uno nuevo.
type CreateAppointmentRequest struct {
	// Campos para la cita
	AppointmentDateTime time.Time `json:"appointmentDateTime" binding:"required"`
	Service             string    `json:"service" binding:"required"` // Mapea al campo 'reason' del modelo Appointment
	Notes               string    `json:"notes"`
	RegistrationStatus  string    `json:"registrationStatus"` // "REGISTERED" o "PENDING_RECORD"

	// Campos para paciente existente
	PatientID uint `json:"patientId,omitempty"`

	// Campos para paciente nuevo (si PatientID es 0)
	PatientFirstName string `json:"patientFirstName,omitempty"`
	PatientLastName  string `json:"patientLastName,omitempty"`
	PatientPhone     string `json:"patientPhone,omitempty"` // Ej: "1234567890"
	PatientEmail     string `json:"patientEmail"`
}

func CreateAppointment(db *gorm.DB, calClient googlecalendar.Client, waClient whatsapp.Client, waTemplates whatsapp.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateAppointmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 1. Obtener ID del Doctor desde el Token
		doctorID := c.MustGet("doctorID").(uint)

		if err := ValidateAppointmentAgainstSchedule(db, doctorID, req.AppointmentDateTime); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var patient models.Patient

		// 2. Lógica de Paciente (Existente o Nuevo)
		if req.PatientID != 0 { // Si se proporciona PatientID, es un paciente existente
			// Verificamos que el paciente ya esté vinculado a ESTE doctor antes de usarlo.
			// Sin este filtro, cualquier doctor autenticado podría adivinar un PatientID
			// ajeno y quedar vinculado a un paciente que no es suyo (IDOR).
			if err := db.Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
				Where("patients.id = ? AND doctor_patients.doctor_id = ?", req.PatientID, doctorID).
				First(&patient).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Paciente existente no encontrado"})
				return
			}
		} else { // Si no se proporciona PatientID, es un paciente nuevo (registro rápido)
			if req.PatientFirstName == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Nombre del paciente es requerido para nuevo registro"})
				return
			}

			patient = models.Patient{
				FirstName: req.PatientFirstName,
				LastName:  req.PatientLastName,
				Phone:     req.PatientPhone,
				Email:     req.PatientEmail,
				// Email, BirthDate, Gender no se envían en el flujo de registro rápido del frontend
			}
			if err := db.Create(&patient).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al crear el paciente"})
				return
			}
		}

		// 3. Vincular al paciente con el doctor si aún no lo está
		// Usamos una transacción para asegurar que la vinculación sea atómica
		// (La creación del paciente ya está manejada fuera de esta transacción si es nuevo)
		err := db.Transaction(func(tx *gorm.DB) error {
			var count int64
			tx.Table("doctor_patients").Where("doctor_id = ? AND patient_id = ?", doctorID, patient.ID).Count(&count)
			if count == 0 {
				var doctor models.Doctor
				doctor.ID = doctorID
				return tx.Model(&doctor).Association("Patients").Append(&patient)
			}
			return nil // Ya está vinculado
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al vincular paciente con el doctor"})
			return
		}

		// 3. Crear la Cita Privada
		appointment := models.Appointment{
			PatientID:           patient.ID,
			DoctorID:            doctorID,
			AppointmentDateTime: req.AppointmentDateTime,
			Reason:              req.Service,            // Mapeamos 'Service' del request a 'Reason' del modelo
			Notes:               req.Notes,              // Asegúrate de agregar Notes al struct Appointment
			Status:              "PENDING",              // Estado inicial por defecto
			RegistrationStatus:  req.RegistrationStatus, // Asegúrate de agregar este campo al struct Appointment
			Source:              "DOCTOR",
		}

		if err := db.Create(&appointment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo agendar la cita"})
			return
		}

		// Cargamos los datos del paciente para que la respuesta JSON sea completa
		db.Preload("Patient").First(&appointment, appointment.ID)

		syncAppointmentToGoogleCalendar(c.Request.Context(), db, calClient, &appointment, patient)

		audit.Log(db, c, doctorID, &patient.ID, audit.ActionCreated, audit.EntityAppointment, appointment.ID, "Cita agendada")

		var doctor models.Doctor
		if db.First(&doctor, doctorID).Error == nil {
			// El link de "sube tus documentos antes de la cita" se genera
			// desde ya (en vez de solo cuando el doctor lo pide a mano desde
			// ConsultationManager) para poder incluirlo directo en el
			// WhatsApp de confirmación de abajo.
			uploadToken, tokenErr := ensureAppointmentUploadToken(db, &appointment)

			// En segundo plano: mismo patrón que ConfirmAppointment/
			// CreatePublicAppointment — no debe bloquear la respuesta al
			// doctor ni usar el contexto de la petición (que muere en
			// cuanto esta función regresa).
			go func(doctor models.Doctor, patient models.Patient, appointment models.Appointment, uploadToken string, tokenErr error) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("⚠️ Pánico recuperado al mandar el aviso de cita agendada %d: %v", appointment.ID, r)
					}
				}()
				if tokenErr != nil {
					log.Printf("⚠️ No se pudo generar el link de documentos para la cita %d: %v", appointment.ID, tokenErr)
				}
				// Reutiliza el mismo mensaje/plantilla que ya usa
				// ConfirmAppointment ("tu cita quedó confirmada") en vez de
				// un texto y una plantilla de Twilio propios: una cita
				// agendada directo por el doctor queda confirmada de una
				// vez, así que es exactamente el mismo aviso — y evita
				// depender de una plantilla nueva (AppointmentBookedByDoctor)
				// que nunca se dio de alta en Twilio/Render y que, al caer
				// a texto libre, WhatsApp puede rechazar en silencio si el
				// paciente nunca le ha escrito antes al número del negocio.
				sendAppointmentDecisionWhatsApp(context.Background(), waClient, waTemplates, doctor, patient, appointment, true, uploadToken)
			}(doctor, patient, appointment, uploadToken, tokenErr)
		}

		c.JSON(http.StatusCreated, appointment)
	}
}

// UploadDocuments maneja la carga de archivos para una cita específica.
// Ruta: /api/appointments/:id/documents
func UploadDocuments(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		appointmentID := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		// 1. Validar existencia y propiedad de la cita
		var appointment models.Appointment
		if err := db.Where("id = ? AND doctor_id = ?", appointmentID, doctorID).First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		// 2. Procesar Multipart Form
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error al procesar los archivos"})
			return
		}

		files := form.File["files"]
		isPrescription := c.PostForm("isPrescription") == "true"

		for _, fileHeader := range files {
			if err := storage.ValidateUploadedFile(fileHeader, storage.UploadKindMedicalDocument); err != nil {
				log.Printf("⚠️ Documento %s rechazado: %v", fileHeader.Filename, err)
				continue
			}

			// Generamos un nombre de archivo propio a partir de la extensión únicamente.
			// Nunca usamos el nombre que manda el cliente para construir la key:
			// eso permitiría path traversal (ej. "../../main.go") o sobrescribir archivos.
			ext := filepath.Ext(filepath.Base(fileHeader.Filename))
			key := fmt.Sprintf("documents/%s_%d%s", appointmentID, time.Now().UnixNano(), ext)

			// 3. Guardar el archivo (disco local o S3, según configuración)
			storedRef, err := storageClient.Save(c.Request.Context(), key, fileHeader)
			if err != nil {
				log.Printf("⚠️ No se pudo guardar el documento %s: %v", fileHeader.Filename, err)
				continue
			}

			doc := models.MedicalDocument{
				FileName:      fileHeader.Filename,
				FileType:      fileHeader.Header.Get("Content-Type"),
				FilePath:      storedRef,
				AppointmentID: appointment.ID,
				Prescription:  isPrescription,
			}

			if err := db.Create(&doc).Error; err != nil {
				// Si falla el registro en la BD, borramos el archivo ya subido para no dejarlo huérfano.
				storageClient.Delete(c.Request.Context(), storedRef)
				continue
			}

			audit.Log(db, c, doctorID, &appointment.PatientID, audit.ActionCreated, audit.EntityMedicalDocument, doc.ID, "Documento clínico subido: "+doc.FileName)
		}

		c.JSON(http.StatusOK, gin.H{"message": "Documentos guardados en servidor exitosamente"})
	}
}

// validAppointmentStatuses son los únicos valores de status que el sistema
// asigna hoy (ver CreateAppointment, CancelAppointment, ExecNightClosure y
// ConsultationForm) — se usan para validar el filtro ?status= y así rechazar
// typos con un 400 claro en vez de devolver una lista vacía sin explicación.
var validAppointmentStatuses = map[string]bool{
	"PENDING":              true,
	"COMPLETED":            true,
	"CANCELLED":            true,
	"NOSHOW":               true,
	"PENDING_CONFIRMATION": true,
}

// Capturamos los parámetros de la URL: /api/appointments?start=...&end=...&status=...
func GetAppointments(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		startDate := c.Query("start")
		endDate := c.Query("end")
		status := c.Query("status")

		var appointments []models.Appointment
		query := db.Where("doctor_id = ?", doctorID).Preload("Patient")

		// Si el usuario envió fechas, filtramos. Antes, una fecha mal formada
		// se ignoraba en silencio (el filtro simplemente no se aplicaba); ahora
		// se rechaza con 400 para que el error no pase desapercibido.
		if startDate != "" && endDate != "" {
			start, err := time.Parse("2006-01-02", startDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Parámetro 'start' inválido, formato esperado YYYY-MM-DD"})
				return
			}
			end, err := time.Parse("2006-01-02", endDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Parámetro 'end' inválido, formato esperado YYYY-MM-DD"})
				return
			}
			end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			query = query.Where("appointment_date_time BETWEEN ? AND ?", start, end)
		} else if startDate != "" || endDate != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Debes enviar tanto 'start' como 'end', o ninguno de los dos"})
			return
		}

		if status != "" {
			if !validAppointmentStatuses[status] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Parámetro 'status' inválido. Valores permitidos: PENDING, COMPLETED, CANCELLED, NOSHOW, PENDING_CONFIRMATION"})
				return
			}
			query = query.Where("status = ?", status)
		} else {
			// Sin filtro explícito, las solicitudes públicas aún sin
			// confirmar (PENDING_CONFIRMATION) no cuentan como cita real
			// todavía — no deben aparecer en la agenda/calendario general,
			// solo en la bandeja dedicada de solicitudes nuevas.
			query = query.Where("status != ?", "PENDING_CONFIRMATION")
		}

		query.Order("appointment_date_time ASC").Find(&appointments)
		presignAppointmentsFiles(c.Request.Context(), storageClient, appointments)
		c.JSON(http.StatusOK, appointments)
	}
}

func GetAppointmentDetail(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var appointment models.Appointment

		// Preload utiliza el nombre del campo en el Struct de Go, no el nombre de la tabla SQL
		err := db.Preload("Patient").
			Preload("Patient.MedicalHistory").
			Preload("MedicalDocuments"). // <--- Carga los documentos asociados por appointment_id
			Where("id = ? AND doctor_id = ?", id, doctorID).
			First(&appointment).Error

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		// Defensa extra: aunque el frontend ya no debería ofrecer "Iniciar
		// Atención" para una solicitud pública sin confirmar, esto bloquea
		// también un acceso directo a la URL de consulta antes de aceptarla.
		if appointment.Status == "PENDING_CONFIRMATION" {
			c.JSON(http.StatusConflict, gin.H{"error": "Esta cita todavía no ha sido confirmada. Confírmala antes de iniciar la consulta."})
			return
		}

		presignAppointmentFiles(c.Request.Context(), storageClient, &appointment)

		audit.Log(db, c, doctorID, &appointment.PatientID, audit.ActionViewed, audit.EntityAppointment, appointment.ID, "Detalle clínico de la cita consultado")

		c.JSON(http.StatusOK, appointment)
	}
}

// GetAppointmentNoteHistory devuelve las versiones anteriores del
// contenido clínico de una cita (diagnóstico/tratamiento/notas), tal como
// quedaron preservadas justo antes de cada edición — ver
// models.AppointmentNoteHistory y el guardado en UpdateAppointment. Solo
// el doctor: es la misma información sensible que el detalle clínico.
func GetAppointmentNoteHistory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var appointment models.Appointment
		if err := db.Select("id").Where("id = ? AND doctor_id = ?", id, doctorID).First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		var history []models.AppointmentNoteHistory
		if err := db.Where("appointment_id = ?", appointment.ID).
			Order("created_at DESC").
			Find(&history).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo consultar el historial de la cita"})
			return
		}

		c.JSON(http.StatusOK, history)
	}
}

// CancelAppointment marca la cita como CANCELLED en vez de borrarla, para
// conservar el historial clínico (misma cita, distinto estado).
func CancelAppointment(db *gorm.DB, calClient googlecalendar.Client, waClient whatsapp.Client, waTemplates whatsapp.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var appointment models.Appointment
		if err := db.Where("id = ? AND doctor_id = ?", id, doctorID).First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}
		wasPendingConfirmation := appointment.Status == "PENDING_CONFIRMATION"

		result := db.Model(&models.Appointment{}).
			Where("id = ? AND doctor_id = ?", id, doctorID).
			Update("status", "CANCELLED")

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cancelar la cita"})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		deleteAppointmentFromGoogleCalendar(c.Request.Context(), db, calClient, appointment)

		// Dos avisos distintos según qué se está cancelando en realidad — en
		// segundo plano en ambos casos: ver el comentario largo en
		// CreatePublicAppointment sobre por qué esto no debe bloquear la
		// respuesta ni usar el contexto de la petición.
		var doctor models.Doctor
		var patient models.Patient
		if db.First(&doctor, doctorID).Error == nil && db.First(&patient, appointment.PatientID).Error == nil {
			if wasPendingConfirmation {
				// Se está rechazando una solicitud en línea todavía sin
				// confirmar ("no pudimos aceptar tu solicitud").
				go func(doctor models.Doctor, patient models.Patient, appointment models.Appointment) {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("⚠️ Pánico recuperado al mandar el aviso de rechazo de la cita %d: %v", appointment.ID, r)
						}
					}()
					patientNotified := sendAppointmentDecisionWhatsApp(context.Background(), waClient, waTemplates, doctor, patient, appointment, false, "")
					sendAppointmentDecisionEmail(doctor, patient, appointment, false, patientNotified)
				}(doctor, patient, appointment)
			} else {
				// Se está cancelando una cita que YA estaba confirmada —
				// aviso distinto ("tu cita fue cancelada"), no el de
				// "rechazamos tu solicitud" de arriba.
				go func(doctor models.Doctor, patient models.Patient, appointment models.Appointment) {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("⚠️ Pánico recuperado al mandar el aviso de cancelación de la cita %d: %v", appointment.ID, r)
						}
					}()
					patientNotified := sendAppointmentCancelledWhatsApp(context.Background(), waClient, waTemplates, doctor, patient, appointment)
					sendAppointmentCancelledEmail(doctor, patient, appointment, patientNotified)
				}(doctor, patient, appointment)
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Cita cancelada", "status": "CANCELLED"})
	}
}

// ConfirmAppointment acepta una solicitud de cita agendada públicamente
// (status PENDING_CONFIRMATION, ver CreatePublicAppointment) y la vuelve
// una cita normal. Solo el doctor/personal del consultorio puede
// confirmarla, y solo si de verdad estaba pendiente de confirmación
// (evita "confirmar" por error una cita ya cancelada o completada).
func ConfirmAppointment(db *gorm.DB, calClient googlecalendar.Client, waClient whatsapp.Client, waTemplates whatsapp.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var appointment models.Appointment
		if err := db.Where("id = ? AND doctor_id = ? AND status = ?", id, doctorID, "PENDING_CONFIRMATION").
			First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No hay ninguna solicitud pendiente con ese ID"})
			return
		}

		if err := db.Model(&models.Appointment{}).
			Where("id = ? AND doctor_id = ?", id, doctorID).
			Update("status", "PENDING").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo confirmar la cita"})
			return
		}

		var doctor models.Doctor
		var patient models.Patient
		if db.First(&doctor, doctorID).Error == nil && db.First(&patient, appointment.PatientID).Error == nil {
			appointment.Status = "PENDING"
			syncAppointmentToGoogleCalendar(c.Request.Context(), db, calClient, &appointment, patient)

			// Igual que en CreateAppointment: en cuanto la cita queda
			// confirmada de verdad (ya no es solo una solicitud), se genera
			// el link de documentos para poder incluirlo en el WhatsApp de
			// confirmación — antes de este cambio, una cita que llegaba por
			// el directorio público NUNCA incluía este link, solo las que
			// el doctor agendaba directo desde su panel.
			uploadToken, tokenErr := ensureAppointmentUploadToken(db, &appointment)

			// En segundo plano: ver el comentario largo en
			// CreatePublicAppointment sobre por qué esto no debe bloquear la
			// respuesta ni usar el contexto de la petición.
			go func(doctor models.Doctor, patient models.Patient, appointment models.Appointment, uploadToken string, tokenErr error) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("⚠️ Pánico recuperado al mandar el aviso de confirmación de la cita %d: %v", appointment.ID, r)
					}
				}()
				if tokenErr != nil {
					log.Printf("⚠️ No se pudo generar el link de documentos para la cita %d: %v", appointment.ID, tokenErr)
				}
				patientNotified := sendAppointmentDecisionWhatsApp(context.Background(), waClient, waTemplates, doctor, patient, appointment, true, uploadToken)
				sendAppointmentDecisionEmail(doctor, patient, appointment, true, patientNotified)
			}(doctor, patient, appointment, uploadToken, tokenErr)
		}

		c.JSON(http.StatusOK, gin.H{"message": "Cita confirmada", "status": "PENDING"})
	}
}

// sendAppointmentDecisionWhatsApp avisa al paciente, por WhatsApp, si su
// solicitud de cita en línea fue aceptada o rechazada por el consultorio.
// Mejor esfuerzo: nunca bloquea ni revierte la confirmación/cancelación en
// sí si Twilio no está configurado o falla. Devuelve true si el paciente
// quedó notificado por este canal, para que el llamador decida si todavía
// hace falta mandarle el correo equivalente (ver sendAppointmentDecisionEmail).
// uploadToken solo aplica (y solo se usa) cuando confirmed es true — no
// tiene caso mandarle a subir documentos para una cita que se rechazó.
func sendAppointmentDecisionWhatsApp(ctx context.Context, waClient whatsapp.Client, waTemplates whatsapp.Templates, doctor models.Doctor, patient models.Patient, appointment models.Appointment, confirmed bool, uploadToken string) bool {
	if waClient == nil || patient.Phone == "" {
		return false
	}

	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)
	vars := map[string]string{"1": doctor.FullName, "2": when}

	var body, contentSID string
	if confirmed {
		body = fmt.Sprintf("¡Tu cita con Dr(a). %s quedó confirmada para el %s!", doctor.FullName, when)
		if uploadToken != "" {
			uploadURL := fmt.Sprintf("%s/public-upload/%s", frontendRedirectBase(), uploadToken)
			body += fmt.Sprintf(" Antes de tu consulta, súbenos aquí cualquier estudio o documento que quieras que el doctor revise: %s", uploadURL)
			vars["3"] = uploadURL
		}
		// Con el teléfono del doctor en el propio aviso, la mayoría de las
		// dudas se resuelven directo con él/ella en vez de terminar como
		// una respuesta a este WhatsApp que nadie del consultorio ve (ver
		// ReplyToWhatsAppThread — esas respuestas solo las lee el
		// superadmin, no el doctor).
		contactSuffix := ""
		if doctor.Phone != "" {
			contactSuffix = fmt.Sprintf(" al %s", doctor.Phone)
		}
		body += fmt.Sprintf(" Si tienes dudas, comunícate con tu doctor%s.", contactSuffix)
		vars["4"] = contactSuffix
		body += " — ProPatient Clinic"
		contentSID = waTemplates.AppointmentConfirmed
	} else {
		body = fmt.Sprintf(
			"El consultorio de Dr(a). %s no pudo aceptar tu solicitud de cita para el %s. Puedes intentar con otro horario desde el directorio de ProPatient Clinic.",
			doctor.FullName, when,
		)
		contentSID = waTemplates.AppointmentRejected
	}

	if err := whatsapp.SendWithFallback(ctx, waClient, patient.Phone, contentSID, vars, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el WhatsApp de %s al paciente (cita %d): %v",
			map[bool]string{true: "confirmación", false: "rechazo"}[confirmed], appointment.ID, err)
		return false
	}
	return true
}

// sendAppointmentDecisionEmail es el equivalente por correo de
// sendAppointmentDecisionWhatsApp. skipPatientEmail es true cuando el mismo
// aviso ya se le entregó por WhatsApp, para no duplicarlo.
func sendAppointmentDecisionEmail(doctor models.Doctor, patient models.Patient, appointment models.Appointment, confirmed bool, skipPatientEmail bool) {
	if patient.Email == "" || skipPatientEmail {
		return
	}

	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)
	var subject, body string
	if confirmed {
		subject = "¡Tu cita con Dr(a). " + doctor.FullName + " quedó confirmada!"
		body = fmt.Sprintf(
			`<p>Hola %s,</p>
			<p>Tu cita con <strong>Dr(a). %s</strong> quedó <strong>confirmada</strong> para el <strong>%s</strong>.</p>
			<p>— ProPatient Clinic</p>`,
			patient.FirstName, doctor.FullName, when,
		)
	} else {
		subject = "No pudimos confirmar tu solicitud de cita"
		body = fmt.Sprintf(
			`<p>Hola %s,</p>
			<p>El consultorio de <strong>Dr(a). %s</strong> no pudo aceptar tu solicitud de cita para el <strong>%s</strong>.</p>
			<p>Puedes intentar con otro horario desde el directorio de ProPatient Clinic.</p>
			<p>— ProPatient Clinic</p>`,
			patient.FirstName, doctor.FullName, when,
		)
	}

	if err := auth.SendEmail(patient.Email, subject, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el correo de %s al paciente (cita %d): %v",
			map[bool]string{true: "confirmación", false: "rechazo"}[confirmed], appointment.ID, err)
	}
}

// sendAppointmentCancelledWhatsApp avisa al paciente, por WhatsApp, que una
// cita YA CONFIRMADA fue cancelada por el consultorio — distinto del
// "rechazo" de sendAppointmentDecisionWhatsApp, que es para una solicitud
// que nunca llegó a confirmarse. Mismo criterio de mejor esfuerzo; devuelve
// true si el paciente quedó notificado por este canal.
func sendAppointmentCancelledWhatsApp(ctx context.Context, waClient whatsapp.Client, waTemplates whatsapp.Templates, doctor models.Doctor, patient models.Patient, appointment models.Appointment) bool {
	if waClient == nil || patient.Phone == "" {
		return false
	}
	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)
	body := fmt.Sprintf(
		"Tu cita con Dr(a). %s del %s fue cancelada por el consultorio. Si necesitas agendar de nuevo, hazlo desde el directorio de ProPatient Clinic. — ProPatient Clinic",
		doctor.FullName, when,
	)
	vars := map[string]string{"1": doctor.FullName, "2": when}
	if err := whatsapp.SendWithFallback(ctx, waClient, patient.Phone, waTemplates.AppointmentCancelled, vars, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el WhatsApp de cancelación al paciente (cita %d): %v", appointment.ID, err)
		return false
	}
	return true
}

// sendAppointmentCancelledEmail es el equivalente por correo de
// sendAppointmentCancelledWhatsApp; se salta si el paciente ya quedó
// notificado por WhatsApp.
func sendAppointmentCancelledEmail(doctor models.Doctor, patient models.Patient, appointment models.Appointment, skipPatientEmail bool) {
	if patient.Email == "" || skipPatientEmail {
		return
	}
	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)
	subject := "Tu cita con Dr(a). " + doctor.FullName + " fue cancelada"
	body := fmt.Sprintf(
		`<p>Hola %s,</p>
		<p>Tu cita con <strong>Dr(a). %s</strong> del <strong>%s</strong> fue <strong>cancelada</strong> por el consultorio.</p>
		<p>Si necesitas agendar de nuevo, puedes hacerlo desde el directorio de ProPatient Clinic.</p>
		<p>— ProPatient Clinic</p>`,
		patient.FirstName, doctor.FullName, when,
	)
	if err := auth.SendEmail(patient.Email, subject, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el correo de cancelación al paciente (cita %d): %v", appointment.ID, err)
	}
}

// sendAppointmentRescheduledWhatsApp avisa al paciente, por WhatsApp, que
// el consultorio movió la fecha/hora de su cita (ver UpdateAppointment).
// Mismo criterio de mejor esfuerzo que el resto; devuelve true si el
// paciente quedó notificado por este canal.
func sendAppointmentRescheduledWhatsApp(ctx context.Context, waClient whatsapp.Client, waTemplates whatsapp.Templates, doctor models.Doctor, patient models.Patient, appointment models.Appointment) bool {
	if waClient == nil || patient.Phone == "" {
		return false
	}
	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)
	// Si el doctor tiene teléfono configurado en su perfil, se agrega como
	// sufijo de la frase de contacto (" al 555..."); si no, la frase queda
	// sin número en vez de mostrar un hueco en blanco.
	contactSuffix := ""
	if doctor.Phone != "" {
		contactSuffix = fmt.Sprintf(" al %s", doctor.Phone)
	}
	body := fmt.Sprintf(
		"Hola, te informamos que tu cita médica con el Dr. %s ha sido reprogramada.\nNueva fecha y hora: %s.\nSi no estás de acuerdo con la reprogramación, comunícate con tu doctor%s.\nSi tienes dudas o necesitas cambiar la cita, responde a este mensaje.",
		doctor.FullName, when, contactSuffix,
	)
	vars := map[string]string{"1": doctor.FullName, "2": when, "3": contactSuffix}
	if err := whatsapp.SendWithFallback(ctx, waClient, patient.Phone, waTemplates.AppointmentRescheduled, vars, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el WhatsApp de reprogramación al paciente (cita %d): %v", appointment.ID, err)
		return false
	}
	return true
}

// sendAppointmentRescheduledEmail es el equivalente por correo de
// sendAppointmentRescheduledWhatsApp; se salta si el paciente ya quedó
// notificado por WhatsApp.
func sendAppointmentRescheduledEmail(doctor models.Doctor, patient models.Patient, appointment models.Appointment, skipPatientEmail bool) {
	if patient.Email == "" || skipPatientEmail {
		return
	}
	when := auth.FormatSpanishDateTime(appointment.AppointmentDateTime)
	subject := "Tu cita con Dr(a). " + doctor.FullName + " cambió de horario"
	body := fmt.Sprintf(
		`<p>Hola %s,</p>
		<p>Tu cita con <strong>Dr(a). %s</strong> fue <strong>reprogramada</strong>. Nueva fecha y hora: <strong>%s</strong>.</p>
		<p>— ProPatient Clinic</p>`,
		patient.FirstName, doctor.FullName, when,
	)
	if err := auth.SendEmail(patient.Email, subject, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el correo de reprogramación al paciente (cita %d): %v", appointment.ID, err)
	}
}

// dynamicNotesHaveContent revisa si al menos una de las secciones de
// Notas de Consulta (configurables por el doctor, ver sectionsConfig en
// el frontend) tiene texto real — DynamicNotes es un mapa label->texto
// serializado como JSON. Un JSON inválido o vacío se trata como "sin
// contenido", nunca como error (la validación de arriba ya decide si
// eso basta o hace falta el diagnóstico).
func dynamicNotesHaveContent(raw datatypes.JSON) bool {
	if len(raw) == 0 {
		return false
	}
	var notes map[string]string
	if err := json.Unmarshal(raw, &notes); err != nil {
		return false
	}
	for _, v := range notes {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

func UpdateAppointment(db *gorm.DB, calClient googlecalendar.Client, waClient whatsapp.Client, waTemplates whatsapp.Templates) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var appointment models.Appointment
		if err := db.Where("id = ? AND doctor_id = ?", id, doctorID).First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		// Una solicitud pública sin confirmar solo se modifica a través de
		// /confirm o /cancel — no de este endpoint genérico (evita, por
		// ejemplo, reprogramarla o guardarle notas de consulta antes de que
		// el consultorio la haya aceptado).
		if appointment.Status == "PENDING_CONFIRMATION" {
			c.JSON(http.StatusConflict, gin.H{"error": "Esta cita todavía no ha sido confirmada. Confírmala o recházala antes de modificarla."})
			return
		}
		previousDateTime := appointment.AppointmentDateTime
		previousStatus := appointment.Status
		// Copia POR VALOR, no el puntero: json.Unmarshal reutiliza el
		// time.Time ya apuntado por un puntero no-nil existente en vez de
		// asignar uno nuevo, así que una copia de puntero aquí apuntaría al
		// mismo valor que ShouldBindJSON está a punto de sobreescribir —
		// mismo bug ya corregido antes con MedicalHistory (ver
		// patient_handler.go).
		var previousFollowUpDate *time.Time
		if appointment.FollowUpDate != nil {
			t := *appointment.FollowUpDate
			previousFollowUpDate = &t
		}
		// Snapshot del contenido clínico ANTES del bind — son strings (tipos
		// valor), así que esta copia ya es independiente del appointment que
		// ShouldBindJSON está a punto de sobreescribir. Se usa abajo para
		// preservar la versión anterior en AppointmentNoteHistory si de
		// verdad cambió algo (ver NOM-024: ninguna nota clínica se pierde al
		// corregirla).
		previousDiagnosis := appointment.Diagnosis
		previousDiagnosisCode := appointment.DiagnosisCode
		previousTreatmentPlan := appointment.TreatmentPlan
		previousNotes := appointment.Notes
		previousDynamicNotes := string(appointment.DynamicNotes)

		// Leemos los cambios del Body (JSON)
		if err := c.ShouldBindJSON(&appointment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		clinicalContentChanged := appointment.Diagnosis != previousDiagnosis ||
			appointment.DiagnosisCode != previousDiagnosisCode ||
			appointment.TreatmentPlan != previousTreatmentPlan ||
			appointment.Notes != previousNotes ||
			string(appointment.DynamicNotes) != previousDynamicNotes

		// NOM-004: un expediente no puede quedar en blanco. La UI ya deja
		// que el doctor marque secciones de sus Notas de Consulta como
		// obligatorias (ver sectionsConfig en el frontend), pero eso es
		// fácil de saltarse llamando a esta ruta directo — este es el
		// candado real: al finalizar (status pasa a COMPLETED) tiene que
		// haber diagnóstico o al menos una nota clínica con contenido.
		if appointment.Status == "COMPLETED" && strings.TrimSpace(appointment.Diagnosis) == "" && !dynamicNotesHaveContent(appointment.DynamicNotes) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No se puede finalizar la consulta sin diagnóstico ni notas clínicas."})
			return
		}

		// Diagnóstico estructurado (catálogo CIE-10, ver Cie10Code): antes
		// era solo un campo opcional para interoperabilidad futura; ahora se
		// exige al finalizar, para que el expediente quede siempre con un
		// código estandarizado y no dependa de que el doctor haya usado el
		// buscador por su cuenta.
		if appointment.Status == "COMPLETED" && strings.TrimSpace(appointment.DiagnosisCode) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Debes asociar un código CIE-10 al diagnóstico antes de finalizar la consulta."})
			return
		}

		// Solo se valida contra el horario laboral si de verdad se está
		// moviendo la fecha/hora (reprogramar) — un PUT que solo guarda
		// notas de consulta o marca seguimiento no debe fallar por esto.
		if !appointment.AppointmentDateTime.Equal(previousDateTime) {
			if err := ValidateAppointmentAgainstSchedule(db, doctorID, appointment.AppointmentDateTime); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		// Si el contenido clínico cambió, preservamos la versión anterior
		// ANTES de sobreescribirla — append-only, nunca se actualiza ni
		// borra una fila de AppointmentNoteHistory ya escrita (ver NOM-024:
		// ninguna nota firmada se pierde al corregirla, queda como versión
		// anterior consultable). Mejor esfuerzo: un fallo aquí no debe
		// bloquear que el doctor guarde su nota.
		if clinicalContentChanged {
			role, actorID, actorName := audit.CurrentActor(db, c, doctorID)
			history := models.AppointmentNoteHistory{
				AppointmentID:         appointment.ID,
				PreviousDiagnosis:     previousDiagnosis,
				PreviousDiagnosisCode: previousDiagnosisCode,
				PreviousTreatmentPlan: previousTreatmentPlan,
				PreviousNotes:         previousNotes,
				PreviousDynamicNotes:  previousDynamicNotes,
				ChangedByRole:         role,
				ChangedByID:           actorID,
				ChangedByName:         actorName,
				IPAddress:             c.ClientIP(),
			}
			if err := db.Create(&history).Error; err != nil {
				log.Printf("⚠️ No se pudo preservar la versión anterior de las notas de la cita %d: %v", appointment.ID, err)
			}
		}

		// Revisar si GORM arroja algún error al guardar en Postgres
		if err := db.Save(&appointment).Error; err != nil {
			log.Printf("❌ Error de GORM al guardar: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar"})
			return
		}

		auditDetails := "Cita actualizada"
		if clinicalContentChanged {
			auditDetails = "Contenido clínico de la cita actualizado (versión anterior preservada)"
		}
		audit.Log(db, c, doctorID, &appointment.PatientID, audit.ActionUpdated, audit.EntityAppointment, appointment.ID, auditDetails)

		// Solo tocamos Google Calendar y avisamos al paciente cuando de
		// verdad se reprogramó la cita (cambió la fecha/hora) — no en cada
		// PUT parcial (ej. marcar seguimiento, guardar notas de la
		// consulta). El aviso de reprogramación solo aplica a una cita
		// normal (PENDING); no tendría sentido para una ya cancelada o
		// completada.
		if !appointment.AppointmentDateTime.Equal(previousDateTime) {
			var patient models.Patient
			if err := db.First(&patient, appointment.PatientID).Error; err == nil {
				syncAppointmentToGoogleCalendar(c.Request.Context(), db, calClient, &appointment, patient)

				if appointment.Status == "PENDING" {
					var doctor models.Doctor
					if db.First(&doctor, doctorID).Error == nil {
						go func(doctor models.Doctor, patient models.Patient, appointment models.Appointment) {
							defer func() {
								if r := recover(); r != nil {
									log.Printf("⚠️ Pánico recuperado al mandar el aviso de reprogramación de la cita %d: %v", appointment.ID, r)
								}
							}()
							patientNotified := sendAppointmentRescheduledWhatsApp(context.Background(), waClient, waTemplates, doctor, patient, appointment)
							sendAppointmentRescheduledEmail(doctor, patient, appointment, patientNotified)
						}(doctor, patient, appointment)
					}
				}
			}
		}

		// Aviso de seguimiento: solo cuando FollowUpDate pasa de "sin
		// seguimiento" (o de otra fecha) a una fecha nueva — no en cada
		// guardado si ya tenía la misma marcada. En segundo plano: ver el
		// comentario largo en CreatePublicAppointment sobre por qué esto no
		// debe bloquear la respuesta ni usar el contexto de la petición.
		followUpChanged := appointment.FollowUpDate != nil &&
			(previousFollowUpDate == nil || !previousFollowUpDate.Equal(*appointment.FollowUpDate))
		if followUpChanged {
			var doctor models.Doctor
			var patient models.Patient
			if db.First(&doctor, doctorID).Error == nil && db.First(&patient, appointment.PatientID).Error == nil {
				followUpDate := *appointment.FollowUpDate
				go func(doctor models.Doctor, patient models.Patient, followUpDate time.Time) {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("⚠️ Pánico recuperado al mandar el WhatsApp de seguimiento del paciente %d: %v", patient.ID, r)
						}
					}()
					sendFollowUpWhatsApp(context.Background(), waClient, waTemplates, doctor, patient, followUpDate)
				}(doctor, patient, followUpDate)
			}
		}

		// Invitación a reseña: solo la primera vez que la cita pasa a
		// COMPLETED (createReviewInviteIfMissing ya es idempotente por su
		// cuenta gracias al índice único en AppointmentID, pero evitamos la
		// consulta de más si el status no cambió). Se crea la invitación de
		// forma síncrona (es solo un INSERT) y el WhatsApp se manda en
		// segundo plano, mismo criterio que el resto de los avisos.
		if previousStatus != "COMPLETED" && appointment.Status == "COMPLETED" {
			review, err := createReviewInviteIfMissing(db, doctorID, appointment.PatientID, appointment.ID)
			if err != nil {
				log.Printf("⚠️ No se pudo crear la invitación a reseña de la cita %d: %v", appointment.ID, err)
			} else if review != nil {
				var doctor models.Doctor
				var patient models.Patient
				if db.First(&doctor, doctorID).Error == nil && db.First(&patient, appointment.PatientID).Error == nil {
					go func(doctor models.Doctor, patient models.Patient, token string) {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("⚠️ Pánico recuperado al mandar el WhatsApp de solicitud de reseña del paciente %d: %v", patient.ID, r)
							}
						}()
						sendReviewRequestWhatsApp(context.Background(), waClient, waTemplates, doctor, patient, token)
					}(doctor, patient, review.Token)
				}
			}
		}

		c.JSON(http.StatusOK, appointment)
	}
}

// sendFollowUpWhatsApp avisa al paciente, por WhatsApp, cuando el doctor
// marca que le toca agendar una cita de seguimiento/control al terminar una
// consulta (ver ConsultationManager.tsx). Mejor esfuerzo.
func sendFollowUpWhatsApp(ctx context.Context, waClient whatsapp.Client, waTemplates whatsapp.Templates, doctor models.Doctor, patient models.Patient, followUpDate time.Time) {
	if waClient == nil || patient.Phone == "" {
		return
	}
	when := auth.FormatSpanishDateTime(followUpDate)
	body := fmt.Sprintf(
		"Hola %s, Dr(a). %s sugiere agendar tu cita de seguimiento para el %s. Escríbele al consultorio para confirmar el horario. — ProPatient Clinic",
		patient.FirstName, doctor.FullName, when,
	)
	vars := map[string]string{"1": patient.FirstName, "2": doctor.FullName, "3": when}
	if err := whatsapp.SendWithFallback(ctx, waClient, patient.Phone, waTemplates.FollowUp, vars, body); err != nil {
		log.Printf("⚠️ No se pudo enviar el WhatsApp de seguimiento al paciente %d: %v", patient.ID, err)
	}
}

// GetTodaySummary devuelve estadísticas rápidas para la pantalla de inicio
func GetTodaySummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		// 1. Obtener la hora del cliente. Si no viene, usar UTC.
		now := time.Now().UTC()
		clientTime := c.Query("clientTime")
		if clientTime != "" {
			if t, err := time.Parse(time.RFC3339, clientTime); err == nil {
				now = t // t mantiene el offset del cliente (ej. -06:00)
			}
		}

		// 2. Definir el "Día" según el calendario del cliente: inicio y fin
		// del día respetando la zona horaria de "now" (el offset del
		// cliente si vino clientTime, UTC si no). Antes esto comparaba con
		// DATE(appointment_date_time) = clientDateStr, pero DATE() calcula
		// la fecha en la zona horaria de la SESIÓN DE POSTGRES (típicamente
		// UTC), no la del cliente — una cita de las 6pm en un huso UTC-7 se
		// guarda como la 1am UTC del día siguiente, así que DATE() la
		// contaba en el día equivocado y desaparecía de "hoy". Comparar por
		// rango en vez de por fecha calculada evita depender de la zona
		// horaria configurada en la base de datos.
		y, m, d := now.Date()
		startOfDay := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		var stats struct {
			TodayCount        int64                `json:"todayCount"`
			PendingCount      int64                `json:"pendingCount"`
			TodayAppointments []models.Appointment `json:"todayAppointments"`
			NextPatient       *models.Appointment  `json:"nextPatient"`
		}
		// 1. Total de citas agendadas para hoy (independientemente del estado,
		// salvo las solicitudes públicas aún sin confirmar: esas no cuentan
		// como cita real todavía — ver CreatePublicAppointment/ConfirmAppointment).
		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND appointment_date_time >= ? AND appointment_date_time < ? AND status != ?",
				doctorID, startOfDay, endOfDay, "PENDING_CONFIRMATION").
			Count(&stats.TodayCount)

		// 2. Total de citas pendientes generales del doctor (su carga de trabajo total)
		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND status = ?", doctorID, "PENDING").
			Count(&stats.PendingCount)

		// 3. Lista de citas de hoy para la tabla (mismo criterio que el conteo:
		// sin las solicitudes pendientes de confirmar, para que "Iniciar
		// Atención" nunca aparezca disponible antes de que el consultorio
		// acepte la solicitud).
		db.Preload("Patient").
			Where("doctor_id = ? AND appointment_date_time >= ? AND appointment_date_time < ? AND status != ?",
				doctorID, startOfDay, endOfDay, "PENDING_CONFIRMATION").
			Order("appointment_date_time ASC").
			Find(&stats.TodayAppointments)

		// 4. Próximo paciente: Cita pendiente más cercana a partir de "Ahora"
		var nextApp models.Appointment
		err := db.Preload("Patient").
			Where("doctor_id = ? AND status = ? AND appointment_date_time > ?",
				doctorID, "PENDING", now.UTC()). // Comparamos con UTC en la DB
			Order("appointment_date_time ASC").
			First(&nextApp).Error

		if err == nil && nextApp.ID > 0 {
			stats.NextPatient = &nextApp
		} else {
			stats.NextPatient = nil
		}

		c.JSON(http.StatusOK, stats)
	}
}

// GetConsultorioStats devuelve métricas agregadas del consultorio del doctor
// (no de un paciente en particular, ver GetPatientStats para eso): total de
// pacientes, citas del mes en curso por estado, tasa histórica de
// inasistencia y próximas citas pendientes en los siguientes 30 días.
func GetConsultorioStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		now := time.Now().UTC()

		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		endOfMonth := startOfMonth.AddDate(0, 1, 0)

		var stats struct {
			TotalPatients         int64   `json:"totalPatients"`
			AppointmentsThisMonth int64   `json:"appointmentsThisMonth"`
			CompletedThisMonth    int64   `json:"completedThisMonth"`
			CancelledThisMonth    int64   `json:"cancelledThisMonth"`
			NoShowThisMonth       int64   `json:"noShowThisMonth"`
			NoShowRate            float64 `json:"noShowRate"`
			UpcomingAppointments  int64   `json:"upcomingAppointments"`
		}

		db.Table("patients").
			Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
			Where("doctor_patients.doctor_id = ?", doctorID).
			Count(&stats.TotalPatients)

		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND appointment_date_time >= ? AND appointment_date_time < ?", doctorID, startOfMonth, endOfMonth).
			Count(&stats.AppointmentsThisMonth)

		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND status = ? AND appointment_date_time >= ? AND appointment_date_time < ?", doctorID, "COMPLETED", startOfMonth, endOfMonth).
			Count(&stats.CompletedThisMonth)

		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND status = ? AND appointment_date_time >= ? AND appointment_date_time < ?", doctorID, "CANCELLED", startOfMonth, endOfMonth).
			Count(&stats.CancelledThisMonth)

		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND status = ? AND appointment_date_time >= ? AND appointment_date_time < ?", doctorID, "NOSHOW", startOfMonth, endOfMonth).
			Count(&stats.NoShowThisMonth)

		// Tasa histórica de inasistencia: sobre todas las citas ya pasadas
		// (no solo las de este mes), para que no oscile tanto mes a mes.
		var totalPast int64
		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND appointment_date_time < ?", doctorID, now).
			Count(&totalPast)
		if totalPast > 0 {
			var totalNoShow int64
			db.Model(&models.Appointment{}).
				Where("doctor_id = ? AND status = ? AND appointment_date_time < ?", doctorID, "NOSHOW", now).
				Count(&totalNoShow)
			stats.NoShowRate = float64(totalNoShow) / float64(totalPast) * 100
		}

		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND status = ? AND appointment_date_time >= ? AND appointment_date_time <= ?",
				doctorID, "PENDING", now, now.AddDate(0, 0, 30)).
			Count(&stats.UpcomingAppointments)

		c.JSON(http.StatusOK, stats)
	}
}

// GetFollowUps devuelve las consultas ya completadas que el doctor marcó con
// una fecha de seguimiento próxima (dentro de los siguientes 7 días,
// incluyendo las ya vencidas si nunca se agendó la cita de control).
func GetFollowUps(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		cutoff := time.Now().UTC().AddDate(0, 0, 7)

		var appointments []models.Appointment
		err := db.Preload("Patient").
			Where("doctor_id = ? AND follow_up_date IS NOT NULL AND follow_up_date <= ?", doctorID, cutoff).
			Order("follow_up_date ASC").
			Find(&appointments).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener los seguimientos pendientes"})
			return
		}

		c.JSON(http.StatusOK, appointments)
	}
}

// GetUpcomingAppointments devuelve las próximas 5 citas programadas
func GetUpcomingAppointments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		var appointments []models.Appointment
		now := time.Now().UTC()
		// Iniciamos desde el comienzo del día actual para no perder citas de hoy, usando time.Date para mayor precisión
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// 1. Filtramos por DoctorID
		// 2. Solo citas pendientes desde hoy
		// 3. Solo citas que no hayan sido canceladas o completadas (opcional, según tu lógica)
		// 4. Ordenamos por fecha ascendente (la más cercana primero)
		// 5. Limitamos a 5 resultados
		err := db.Preload("Patient").
			Where("doctor_id = ? AND status = ? AND appointment_date_time >= ?",
				doctorID, "PENDING", startOfDay).
			Order("appointment_date_time ASC").
			Limit(5).
			Find(&appointments).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener las próximas citas"})
			return
		}

		c.JSON(http.StatusOK, appointments)
	}
}

func UpdateAppointmentDocument(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		// 1. Obtener y validar el ID del documento desde los parámetros de la ruta
		docIDStr := c.Param("docId")
		docID, err := strconv.Atoi(docIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID de documento inválido"})
			return
		}

		// 2. Parsear el cuerpo de la petición (JSON)
		var input models.UpdateDocumentInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El campo filename es obligatorio"})
			return
		}

		// 3. Verificar que el documento pertenece a una cita del doctor autenticado
		// antes de tocarlo (evita que un doctor edite documentos de otro).
		var doc models.MedicalDocument
		err = db.Joins("JOIN appointments ON appointments.id = medical_documents.appointment_id").
			Where("medical_documents.id = ? AND appointments.doctor_id = ?", docID, doctorID).
			First(&doc).Error
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Documento no encontrado"})
			return
		}

		// 4. Ejecutar la actualización en la base de datos
		err = db.Model(&models.MedicalDocument{}).
			Where("id = ?", docID).
			Update("file_name", input.Filename).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar el nombre del archivo"})
			return
		}

		// 4. Responder con éxito
		c.JSON(http.StatusOK, gin.H{
			"message":  "Nombre del documento actualizado con éxito",
			"filename": input.Filename,
		})
	}
}

func SaveRecipePDF(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		// 0. Verificar que la cita pertenece al doctor autenticado
		var appointment models.Appointment
		if err := db.Where("id = ? AND doctor_id = ?", id, doctorID).First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		// 1. Extraer el archivo binario enviado desde el FormData
		file, err := c.FormFile("recipe_pdf")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No se recibió ningún archivo PDF"})
			return
		}

		if err := storage.ValidateUploadedFile(file, storage.UploadKindRecipePDF); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Generamos el nombre nosotros mismos (nunca confiar en el filename del cliente
		// para construir la key: riesgo de path traversal/sobrescritura).
		ext := filepath.Ext(filepath.Base(file.Filename))
		if ext == "" {
			ext = ".pdf"
		}
		key := fmt.Sprintf("recipes/receta_%s_%d%s", id, time.Now().UnixNano(), ext)

		// 2. Guardar el archivo (disco local o S3, según configuración)
		storedRef, err := storageClient.Save(c.Request.Context(), key, file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar el archivo"})
			return
		}

		// 3. Actualizar la columna RecipePDFPath con la referencia guardada
		// (path público en disco local, key de objeto en S3).
		err = db.Model(&models.Appointment{}).
			Where("id = ? AND doctor_id = ?", id, doctorID).
			Update("recipe_pdf_path", storedRef).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar la cita en la base de datos"})
			return
		}

		// storedRef es solo la key en S3 (bucket privado, no sirve como URL
		// directa) — hay que resolverla a una URL firmada antes de
		// devolverla, mismo paso que ya hace presignAppointmentFiles para
		// cuando se consulta la cita después. Sin esto, el frontend recibía
		// la key cruda y armaba una URL que no existe (404).
		recipePdfURL := storedRef
		if url, err := storageClient.URL(c.Request.Context(), storedRef); err == nil {
			recipePdfURL = url
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Receta en PDF almacenada y asociada correctamente",
			"recipePdfPath": recipePdfURL,
		})
	}
}

// GetOrAssignRecipeNumber devuelve el folio de receta de esta cita — ver
// Appointment.RecipeNumber. Si la cita ya tenía uno (de una generación
// anterior de la MISMA receta), regresa ese mismo número; si es la primera
// vez, lo toma del contador propio del doctor (Doctor.LastRecipeNumber),
// incrementándolo de forma atómica dentro de una transacción con bloqueo
// de fila — así dos peticiones casi simultáneas (doble clic, dos pestañas)
// nunca terminan asignando el mismo folio a dos recetas distintas. El
// frontend llama esto ANTES de armar el PDF (ver
// utils/recipePdf.generateAndSaveRecipePDF), para poder imprimir el folio
// dentro de la propia receta.
func GetOrAssignRecipeNumber(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var appointment models.Appointment
		if err := db.Where("id = ? AND doctor_id = ?", id, doctorID).First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		if appointment.RecipeNumber != nil {
			c.JSON(http.StatusOK, gin.H{"recipeNumber": *appointment.RecipeNumber})
			return
		}

		var recipeNumber uint
		err := db.Transaction(func(tx *gorm.DB) error {
			var doctor models.Doctor
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&doctor, doctorID).Error; err != nil {
				return err
			}
			recipeNumber = doctor.LastRecipeNumber + 1
			if err := tx.Model(&doctor).Update("last_recipe_number", recipeNumber).Error; err != nil {
				return err
			}
			return tx.Model(&models.Appointment{}).
				Where("id = ? AND doctor_id = ?", id, doctorID).
				Update("recipe_number", recipeNumber).Error
		})
		if err != nil {
			log.Printf("⚠️ No se pudo asignar folio de receta a la cita %s (doctor %d): %v", id, doctorID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo asignar el folio de la receta"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"recipeNumber": recipeNumber})
	}
}
