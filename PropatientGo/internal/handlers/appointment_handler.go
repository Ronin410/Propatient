package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"propatient-api/internal/models"
	"strconv"
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

		files := form.File["files"]
		isPrescription := c.PostForm("isPrescription") == "true"

		// Definir y crear la carpeta de destino local si no existe
		uploadDir := "./uploads"
		if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo crear el directorio de subidas"})
			return
		}

		for _, fileHeader := range files {
			// Generamos un nombre de archivo propio a partir de la extensión únicamente.
			// Nunca usamos el nombre que manda el cliente para construir la ruta en disco:
			// eso permitiría path traversal (ej. "../../main.go") o sobrescribir archivos.
			ext := filepath.Ext(filepath.Base(fileHeader.Filename))
			uniqueFileName := fmt.Sprintf("%s_%d%s", appointmentID, time.Now().UnixNano(), ext)
			dst := filepath.Join(uploadDir, uniqueFileName)

			// 3. Guardar el archivo físicamente en la carpeta del Backend
			if err := c.SaveUploadedFile(fileHeader, dst); err != nil {
				// Si falla el guardado físico en disco, pasamos al siguiente
				continue
			}

			// 4. Definir la URL pública que se guardará en la BD.
			// Debe coincidir con r.Static("/uploads", "./uploads") en main.go.
			fileURL := fmt.Sprintf("/uploads/%s", uniqueFileName)

			doc := models.MedicalDocument{
				FileName:      fileHeader.Filename,
				FileType:      fileHeader.Header.Get("Content-Type"),
				FilePath:      fileURL, // <--- Guardamos la URL pública en lugar de los bytes
				AppointmentID: appointment.ID,
				Prescription:  isPrescription,
			}

			if err := db.Create(&doc).Error; err != nil {
				// Si falla el registro en la BD, podrías opcionalmente borrar el archivo físico del disco
				os.Remove(dst)
				continue
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Documentos guardados en servidor exitosamente"})
	}
}

// validAppointmentStatuses son los únicos valores de status que el sistema
// asigna hoy (ver CreateAppointment, CancelAppointment, ExecNightClosure y
// ConsultationForm) — se usan para validar el filtro ?status= y así rechazar
// typos con un 400 claro en vez de devolver una lista vacía sin explicación.
var validAppointmentStatuses = map[string]bool{
	"PENDING":   true,
	"COMPLETED": true,
	"CANCELLED": true,
	"NOSHOW":    true,
}

// Capturamos los parámetros de la URL: /api/appointments?start=...&end=...&status=...
func GetAppointments(db *gorm.DB) gin.HandlerFunc {
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
				c.JSON(http.StatusBadRequest, gin.H{"error": "Parámetro 'status' inválido. Valores permitidos: PENDING, COMPLETED, CANCELLED, NOSHOW"})
				return
			}
			query = query.Where("status = ?", status)
		}

		query.Order("appointment_date_time ASC").Find(&appointments)
		c.JSON(http.StatusOK, appointments)
	}
}

func GetAppointmentDetail(db *gorm.DB) gin.HandlerFunc {
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

		c.JSON(http.StatusOK, appointment)
	}
}

// CancelAppointment marca la cita como CANCELLED en vez de borrarla, para
// conservar el historial clínico (misma cita, distinto estado).
func CancelAppointment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

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

		c.JSON(http.StatusOK, gin.H{"message": "Cita cancelada", "status": "CANCELLED"})
	}
}

func UpdateAppointment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		// --- BLOQUE DE DEBBUGGING: LEER JSON CRUDO ---
		// Leemos los bytes directos que vienen de la petición HTTP
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		// Volvemos a restaurar el Body para que ShouldBindJSON pueda leerlo después
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Imprimimos en la consola de la terminal lo que el frontend mandó
		log.Printf("📥 JSON recibido desde el Frontend: %s", string(bodyBytes))
		// ----------------------------------------------

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

		// Revisar si GORM arroja algún error al guardar en Postgres
		if err := db.Save(&appointment).Error; err != nil {
			log.Printf("❌ Error de GORM al guardar: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar"})
			return
		}

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
		//y, m, d := now.Date()
		//startOfDay := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
		//endOfDay := startOfDay.Add(24 * time.Hour)

		var stats struct {
			TodayCount        int64                `json:"todayCount"`
			PendingCount      int64                `json:"pendingCount"`
			TodayAppointments []models.Appointment `json:"todayAppointments"`
			NextPatient       *models.Appointment  `json:"nextPatient"`
		}
		clientDateStr := now.Format("2006-01-02")
		// 1. Total de citas agendadas para hoy (independientemente del estado)
		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND DATE(appointment_date_time) = ?", doctorID, clientDateStr).
			Count(&stats.TodayCount)

		// 2. Total de citas pendientes generales del doctor (su carga de trabajo total)
		db.Model(&models.Appointment{}).
			Where("doctor_id = ? AND status = ?", doctorID, "PENDING").
			Count(&stats.PendingCount)

		// 3. Lista de citas de hoy para la tabla
		db.Preload("Patient").
			Where("doctor_id = ? AND DATE(appointment_date_time) = ?", doctorID, clientDateStr).
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

func SaveRecipePDF(db *gorm.DB) gin.HandlerFunc {
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

		// 2. Definir una ruta física para guardar las recetas dentro del servidor
		uploadDir := "./uploads/recipes"
		if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo crear el directorio de almacenamiento"})
			return
		}

		// Generamos el nombre nosotros mismos (nunca confiar en el filename del cliente
		// para construir una ruta en disco: riesgo de path traversal/sobrescritura).
		ext := filepath.Ext(filepath.Base(file.Filename))
		if ext == "" {
			ext = ".pdf"
		}
		uniqueFileName := fmt.Sprintf("receta_%s_%d%s", id, time.Now().UnixNano(), ext)
		filePath := filepath.Join(uploadDir, uniqueFileName)

		// 3. Guardar el archivo en el sistema de archivos del servidor/contenedor
		if err := c.SaveUploadedFile(file, filePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al escribir el archivo en disco"})
			return
		}

		// 4. Actualizar la columna RecipePDFPath en tu modelo Appointment usando GORM
		// (Asegúrate de tener el campo RecipePDFPath string `json:"recipePdfPath"` en tu struct)
		err = db.Model(&models.Appointment{}).
			Where("id = ? AND doctor_id = ?", id, doctorID).
			Update("recipe_pdf_path", filePath).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar la cita en la base de datos"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Receta en PDF almacenada y asociada correctamente",
			"path":    filePath,
		})
	}
}
