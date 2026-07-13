package handlers

import (
	"math"
	"net/http"
	"propatient-api/internal/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetPatientHistory recupera las consultas de un paciente, filtradas por el doctor actual
func GetPatientHistory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Obtener el ID del paciente de la URL (/api/patients/:id/history)
		patientID := c.Param("id")

		// 2. Obtener el ID del doctor desde el contexto (inyectado por el Middleware de JWT)
		// Nota: Asegúrate de que tu middleware guarde el "doctorID" al validar el token
		doctorID, exists := c.Get("doctorID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No se pudo identificar al doctor"})
			return
		}

		var appointments []models.Appointment

		// 3. Consulta con Filtro Doble (Privacidad)
		// Buscamos citas que pertenezcan a este paciente Y que hayan sido atendidas por ESTE doctor
		result := db.Where("patient_id = ? AND doctor_id = ?", patientID, doctorID).
			Order("appointment_date_time desc").
			Find(&appointments)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al consultar el historial"})
			return
		}

		// 4. Responder al frontend (React)
		c.JSON(http.StatusOK, appointments)
	}
}

// UpdatePatient permite editar los datos de un paciente
func UpdatePatient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		// 1. Cargamos al paciente Y PRECRAMOS su historial médico actual
		var patient models.Patient
		err := db.Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
			Preload("MedicalHistory"). // <--- CRÍTICO: Carga el MedicalHistory existente con su ID real de la BD
			Where("patients.id = ? AND doctor_patients.doctor_id = ?", id, doctorID).
			First(&patient).Error

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Paciente no encontrado o no tiene permisos"})
			return
		}

		// Si el paciente no tenía un historial creado previamente en la BD (caso raro pero posible),
		// inicializamos la estructura para que no sea un puntero nil al bindear
		if patient.MedicalHistory == nil {
			patient.MedicalHistory = &models.MedicalHistory{}
		}

		// 2. Mapeamos los datos que vienen del frontend sobre el objeto ya cargado.
		// Al hacer esto, los campos de texto cambian, pero los IDs originales de la BD se conservan.
		if err := c.ShouldBindJSON(&patient); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 3. Forzar de forma explícita que el PatientID del historial coincida con el del paciente
		patient.MedicalHistory.PatientID = patient.ID

		// 4. Guardamos todo bajo una sesión de FullSaveAssociations
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Session(&gorm.Session{FullSaveAssociations: true}).Save(&patient).Error; err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar los antecedentes clínicos"})
			return
		}

		c.JSON(http.StatusOK, patient)
	}
}

// GetPatients devuelve, paginados, los pacientes vinculados al doctor logueado
func GetPatients(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
		if err != nil || page < 1 {
			page = 1
		}
		pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
		if err != nil || pageSize < 1 {
			pageSize = 20
		}
		if pageSize > 100 {
			pageSize = 100
		}

		// GORM consume el builder al ejecutar un finisher (Count/Find), así que
		// armamos el Joins/Where dos veces en vez de reutilizar la misma variable.
		var total int64
		if err := db.Table("patients").
			Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
			Where("doctor_patients.doctor_id = ?", doctorID).
			Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al contar pacientes"})
			return
		}

		var patients []models.Patient
		err = db.Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
			Where("doctor_patients.doctor_id = ?", doctorID).
			Order("patients.created_at DESC").
			Limit(pageSize).
			Offset((page - 1) * pageSize).
			Find(&patients).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener pacientes"})
			return
		}

		totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
		c.JSON(http.StatusOK, gin.H{
			"data":       patients,
			"total":      total,
			"page":       page,
			"pageSize":   pageSize,
			"totalPages": totalPages,
		})
	}
}

// RemovePatientFromDoctor desvincula al paciente de ESTE doctor (no borra al
// paciente: es una relación many2many y otros doctores pueden seguir
// teniéndolo vinculado con su propio historial de citas).
func RemovePatientFromDoctor(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		result := db.Exec(
			"DELETE FROM doctor_patients WHERE doctor_id = ? AND patient_id = ?",
			doctorID, id,
		)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al eliminar al paciente de tu lista"})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Paciente no encontrado o no tienes permiso para eliminarlo"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Paciente eliminado de tu lista"})
	}
}

// CreatePatient registra un nuevo paciente y lo vincula al doctor actual
func CreatePatient(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		var patient models.Patient

		if err := c.ShouldBindJSON(&patient); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Iniciamos transacción para asegurar la integridad de la relación
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&patient).Error; err != nil {
				return err
			}

			var doctor models.Doctor
			doctor.ID = doctorID
			// Vincular explícitamente al doctor logueado en la tabla muchos a muchos
			return tx.Model(&doctor).Association("Patients").Append(&patient)
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo registrar al paciente"})
			return
		}

		c.JSON(http.StatusCreated, patient)
	}
}

// GetPatientById devuelve la información de un paciente específico si tiene citas con el doctor
func GetPatientById(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var patient models.Patient

		// Buscamos a través de la tabla de relación doctor_patients
		err := db.Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
			Where("patients.id = ? AND doctor_patients.doctor_id = ?", id, doctorID).
			First(&patient).Error

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Paciente no encontrado o no tienes permiso para verlo"})
			return
		}

		c.JSON(http.StatusOK, patient)
	}
}

func GetPatientMedicalHistory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		patientID := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var patient models.Patient

		// Preload("MedicalHistory"): Trae alergias, antecedentes, etc.
		// Preload("Appointments"): Trae la cronología de consultas filtrada por doctor.
		err := db.Preload("MedicalHistory").
			Preload("Appointments", func(db *gorm.DB) *gorm.DB {
				return db.Where("doctor_id = ?", doctorID).Order("appointment_date_time DESC")
			}).
			Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
			Where("patients.id = ? AND doctor_patients.doctor_id = ?", patientID, doctorID).
			First(&patient).Error

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Historial no encontrado"})
			return
		}

		c.JSON(http.StatusOK, patient)
	}
}

func UpdateMedicalHistory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		patientIDStr := c.Param("id")
		id64, err := strconv.ParseUint(patientIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID de paciente inválido"})
			return
		}
		patientID := uint(id64)
		doctorID := c.MustGet("doctorID").(uint)

		var history models.MedicalHistory
		var patient models.Patient

		// 1. Verificar que el paciente existe y está vinculado al doctor
		err = db.Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
			Where("patients.id = ? AND doctor_patients.doctor_id = ?", patientID, doctorID).
			First(&patient).Error
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Paciente no encontrado o no tienes permiso para actualizar su historial"})
			return
		}

		// 2. Buscar o crear el historial médico para este paciente
		// FirstOrCreate busca por PatientID. Si no existe, crea uno nuevo con PatientID.
		db.Where("patient_id = ?", patientID).FirstOrCreate(&history, models.MedicalHistory{PatientID: patientID})

		// 3. BindJSON para actualizar los campos del historial
		if err := c.ShouldBindJSON(&history); err != nil { // Aquí history ya tiene el ID si existía
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 4. Guardar los cambios
		if err := db.Save(&history).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar el historial médico"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Historial médico actualizado", "history": history})
	}
}

// SearchPatients busca pacientes por nombre o teléfono vinculados al doctor
// SearchPatients permite buscar pacientes por nombre o teléfono vinculados al doctor
func SearchPatients(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("query")
		doctorID := c.MustGet("doctorID").(uint)
		var patients []models.Patient

		if query == "" {
			c.JSON(http.StatusOK, []models.Patient{})
			return
		}

		// Buscamos coincidencias en Nombre, Apellido o Teléfono
		// Usando la tabla intermedia doctor_patients para filtrar por doctor
		searchTerm := "%" + query + "%"
		err := db.Distinct("patients.*").
			Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
			Where("doctor_patients.doctor_id = ? AND (patients.first_name ILIKE ? OR patients.last_name ILIKE ? OR patients.phone LIKE ?)",
				doctorID, searchTerm, searchTerm, searchTerm).
			Limit(10).
			Find(&patients).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error en la búsqueda"})
			return
		}

		c.JSON(http.StatusOK, patients)
	}
}

// GetPatientStats devuelve conteo de citas y fecha de última visita
func GetPatientStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		patientID := c.Param("id")
		doctorID := c.MustGet("doctorID").(uint)

		var stats struct {
			TotalAppointments int64     `json:"total_appointments"`
			LastVisit         time.Time `json:"last_visit"`
		}

		db.Model(&models.Appointment{}).
			Where("patient_id = ? AND doctor_id = ?", patientID, doctorID).
			Count(&stats.TotalAppointments)

		db.Model(&models.Appointment{}).
			Where("patient_id = ? AND doctor_id = ?", patientID, doctorID).
			Select("MAX(appointment_date_time)").
			Row().Scan(&stats.LastVisit)

		c.JSON(http.StatusOK, stats)
	}
}

// Función auxiliar para convertir IDs de string a uint
func castToUint(s string) uint {
	id, _ := strconv.ParseUint(s, 10, 32)
	return uint(id)
}
