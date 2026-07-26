package handlers

import (
	"net/http"
	"time"

	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ExportMyData arma un volcado en JSON de todo lo asociado a la cuenta del
// doctor autenticado (perfil, pacientes, citas, personal) — cumple con el
// derecho de Acceso de las garantías ARCO que promete el Aviso de
// Privacidad. Ruta protegida con RequireDoctorRole: el personal nunca debe
// poder descargar el expediente completo del consultorio.
func ExportMyData(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		// Solo los pacientes vinculados a ESTE doctor (many2many vía
		// doctor_patients) — nunca los de otro consultorio.
		var patients []models.Patient
		db.Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
			Where("doctor_patients.doctor_id = ?", doctorID).
			Preload("MedicalHistory").
			Find(&patients)

		var appointments []models.Appointment
		db.Where("doctor_id = ?", doctorID).Preload("MedicalDocuments").Find(&appointments)

		// Todo el personal con un vínculo (activo o no) a ESTE doctor,
		// vía doctor_staff — Staff ya no tiene una columna doctor_id
		// propia (ver models.DoctorStaff).
		var staff []models.Staff
		db.Joins("JOIN doctor_staff ON doctor_staff.staff_id = staffs.id").
			Where("doctor_staff.doctor_id = ?", doctorID).
			Find(&staff)

		c.Header("Content-Disposition", `attachment; filename="propatient-mis-datos.json"`)
		c.JSON(http.StatusOK, gin.H{
			"exportedAt":   time.Now().UTC(),
			"doctor":       doctor,
			"patients":     patients,
			"appointments": appointments,
			"staff":        staff,
		})
	}
}

// DeleteMyAccount da de baja la cuenta del doctor (soft-delete: GORM marca
// DeletedAt, y tanto LoginHandler como GoogleLoginHandler consultan con
// First(), que ya excluye registros borrados — la cuenta deja de poder
// iniciar sesión sin necesidad de ningún chequeo extra).
//
// A propósito NO se borran en cascada pacientes, citas ni expedientes
// clínicos: son datos de salud con obligación legal de conservación, y
// además los pacientes pueden seguir vinculados a otro doctor (relación
// many2many). Si el doctor necesita el borrado completo tiene que
// solicitarlo por el contacto de privacidad (ver Aviso de Privacidad).
func DeleteMyAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		if doctor.SubscriptionStatus == "active" {
			c.JSON(http.StatusConflict, gin.H{"error": "Tienes una suscripción activa. Cancélala primero desde Facturación antes de eliminar tu cuenta."})
			return
		}

		if err := db.Delete(&doctor).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo eliminar la cuenta"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Tu cuenta fue eliminada. Los expedientes clínicos se conservan por obligación legal; contáctanos si necesitas su eliminación completa.",
		})
	}
}
