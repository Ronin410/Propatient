package handlers

import (
	"net/http"
	"propatient-api/internal/models"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

func CreateAppointment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateAppointmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 1. Obtener ID del Doctor desde el Token
		doctorID := c.MustGet("doctorID").(uint)

		var patient models.Patient

		// 2. Lógica de Paciente (Existente o Nuevo)
		if req.PatientID != 0 { // Si se proporciona PatientID, es un paciente existente
			if err := db.First(&patient, req.PatientID).Error; err != nil {
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
		}

		if err := db.Create(&appointment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo agendar la cita"})
			return
		}

		// Cargamos los datos del paciente para que la respuesta JSON sea completa
		db.Preload("Patient").First(&appointment, appointment.ID)

		c.JSON(http.StatusCreated, appointment)
	}
}

// UploadDocuments maneja la carga de archivos para una cita específica.
// Ruta: /api/appointments/:id/documents
func UploadDocuments(db *gorm.DB) gin.HandlerFunc {
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

		files := form.File["files"] // El frontend envía el campo 'files'
		isPrescription := c.PostForm("isPrescription") == "true"

		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				continue
			}
			defer file.Close()

			// Leer el contenido del archivo a bytes
			data := make([]byte, fileHeader.Size)
			_, err = file.Read(data)
			if err != nil {
				continue
			}

			doc := models.MedicalDocument{
				FileName:      fileHeader.Filename,
				FileType:      fileHeader.Header.Get("Content-Type"),
				Data:          data,
				AppointmentID: appointment.ID,
				Prescription:  isPrescription,
			}

			if err := db.Create(&doc).Error; err != nil {
				// Si falla un archivo, continuamos con el siguiente
				continue
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Documentos guardados exitosamente"})
	}
}

func GetAppointments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		// Capturamos los parámetros de la URL: /api/appointments?start=...&end=...
		startDate := c.Query("start")
		endDate := c.Query("end")

		var appointments []models.Appointment
		query := db.Where("doctor_id = ?", doctorID).Preload("Patient")

		// Si el usuario envió fechas, filtramos
		if startDate != "" && endDate != "" {
			start, _ := time.Parse("2006-01-02", startDate)
			end, _ := time.Parse("2006-01-02", endDate)
			end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			query = query.Where("appointment_date_time BETWEEN ? AND ?", start, end)
		}

		query.Find(&appointments)
		c.JSON(http.StatusOK, appointments)
	}
}

func GetAppointmentDetail(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var appointment models.Appointment
		// Preload("Patient") trae los datos del dueño de la cita
		if err := db.Preload("Patient").Where("id = ? AND doctor_id = ?", id, doctorID).First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		c.JSON(http.StatusOK, appointment)
	}
}

func UpdateAppointment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var appointment models.Appointment
		if err := db.Where("id = ? AND doctor_id = ?", id, doctorID).First(&appointment).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Cita no encontrada"})
			return
		}

		// Leemos los cambios del Body (JSON)
		if err := c.ShouldBindJSON(&appointment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		db.Save(&appointment)
		c.JSON(http.StatusOK, appointment)
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

		// 2. Definir el "Día" según el calendario del cliente
		// Creamos el inicio y fin del día respetando la zona horaria donde está el doctor
		y, m, d := now.Date()
		startOfDay := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		var stats struct {
			TodayCount        int64                `json:"todayCount"`
			PendingCount      int64                `json:"pendingCount"`
			TodayAppointments []models.Appointment `json:"todayAppointments"`
			NextPatient       *models.Appointment  `json:"nextPatient"`
		}

		// 1. Total de citas agendadas para hoy (independientemente del estado)
		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND appointment_date_time >= ? AND appointment_date_time < ?",
				doctorID, startOfDay.UTC(), endOfDay.UTC()).
			Count(&stats.TodayCount)

		// 2. Total de citas pendientes generales del doctor (su carga de trabajo total)
		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND status = ?", doctorID, "PENDING").
			Count(&stats.PendingCount)

		// 3. Lista de citas de hoy para la tabla
		db.Preload("Patient").
			Where("doctor_id = ? AND appointment_date_time >= ? AND appointment_date_time < ?",
				doctorID, startOfDay.UTC(), endOfDay.UTC()).
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
