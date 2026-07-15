package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"propatient-api/internal/models"
	"propatient-api/internal/storage"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PublicDoctorSummary es el subconjunto de campos de un doctor que se
// expone en el directorio público — nunca el modelo Doctor completo (aunque
// hoy ya protege lo sensible con json:"-", usar un struct dedicado evita
// que un campo nuevo se filtre por accidente el día que alguien lo agregue
// al modelo).
type PublicDoctorSummary struct {
	ID               uint     `json:"id"`
	FullName         string   `json:"fullName"`
	MedicalSpecialty string   `json:"medicalSpecialty"`
	PublicBio        string   `json:"publicBio"`
	AvatarUrl        string   `json:"avatarUrl"`
	Address          string   `json:"address"`
	Phone            string   `json:"phone"`
	Latitude         *float64 `json:"latitude"`
	Longitude        *float64 `json:"longitude"`
	PublicSlug       string   `json:"publicSlug"`
}

func toPublicDoctorSummary(d models.Doctor) PublicDoctorSummary {
	return PublicDoctorSummary{
		ID:               d.ID,
		FullName:         d.FullName,
		MedicalSpecialty: d.MedicalSpecialty,
		PublicBio:        d.PublicBio,
		AvatarUrl:        d.AvatarUrl,
		Address:          d.Address,
		Phone:            d.Phone,
		Latitude:         d.Latitude,
		Longitude:        d.Longitude,
		PublicSlug:       d.PublicSlug,
	}
}

// isDoctorAcceptingPublicBookings replica la misma regla de
// billing.RequireActiveSubscription (prueba vigente o suscripción activa),
// para no exponer en el directorio ni dejar agendar con un consultorio que
// ya no tiene acceso al sistema.
func isDoctorAcceptingPublicBookings(d models.Doctor) bool {
	if d.SubscriptionStatus == "active" {
		return true
	}
	return d.SubscriptionStatus == "trialing" && d.TrialEndsAt != nil && time.Now().UTC().Before(*d.TrialEndsAt)
}

// GetPublicDoctors devuelve el directorio de doctores que activaron su
// listado público. Sin autenticación.
func GetPublicDoctors(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var doctors []models.Doctor
		if err := db.Where("public_listed = ?", true).Find(&doctors).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al obtener el directorio de doctores"})
			return
		}

		summaries := make([]PublicDoctorSummary, 0, len(doctors))
		for i := range doctors {
			if !isDoctorAcceptingPublicBookings(doctors[i]) {
				continue
			}
			presignDoctorFiles(c.Request.Context(), storageClient, &doctors[i])
			summaries = append(summaries, toPublicDoctorSummary(doctors[i]))
		}

		c.JSON(http.StatusOK, summaries)
	}
}

// GetPublicDoctorBySlug devuelve el perfil público de un solo doctor, para
// la página /dr/:slug. Sin autenticación.
func GetPublicDoctorBySlug(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")

		var doctor models.Doctor
		if err := db.Where("public_slug = ? AND public_listed = ?", slug, true).First(&doctor).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if !isDoctorAcceptingPublicBookings(doctor) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		presignDoctorFiles(c.Request.Context(), storageClient, &doctor)
		c.JSON(http.StatusOK, toPublicDoctorSummary(doctor))
	}
}

type publicAppointmentRequest struct {
	DoctorID            uint      `json:"doctorId" binding:"required"`
	AppointmentDateTime time.Time `json:"appointmentDateTime" binding:"required"`
	Reason              string    `json:"reason"`
	PatientFirstName    string    `json:"patientFirstName" binding:"required"`
	PatientLastName     string    `json:"patientLastName" binding:"required"`
	PatientPhone        string    `json:"patientPhone" binding:"required"`
	PatientEmail        string    `json:"patientEmail" binding:"required,email"`
}

// CreatePublicAppointment permite a cualquier persona (sin cuenta) solicitar
// una cita con un doctor del directorio público. La cita nace en estado
// PENDING_CONFIRMATION: no aparece como una cita real hasta que el doctor
// (o su personal) la confirma con ConfirmAppointment — así una solicitud
// falsa o spam nunca reserva un horario de verdad sin que alguien la revise.
func CreatePublicAppointment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req publicAppointmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var doctor models.Doctor
		if err := db.Where("id = ? AND public_listed = ?", req.DoctorID, true).First(&doctor).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Este doctor no está disponible para agendar citas en línea"})
			return
		}
		if !isDoctorAcceptingPublicBookings(doctor) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Este consultorio no está aceptando citas en este momento"})
			return
		}

		patient, alreadyLinked, err := findOrCreatePublicBookingPatient(db, doctor, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo registrar tus datos"})
			return
		}

		if !alreadyLinked {
			if err := db.Model(&doctor).Association("Patients").Append(patient); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo vincular tu solicitud con el consultorio"})
				return
			}
		}

		appointment := models.Appointment{
			PatientID:           patient.ID,
			DoctorID:            doctor.ID,
			AppointmentDateTime: req.AppointmentDateTime,
			Reason:              req.Reason,
			Status:              "PENDING_CONFIRMATION",
			RegistrationStatus:  "PENDING_RECORD",
		}
		if err := db.Create(&appointment).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo registrar tu solicitud de cita"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "Tu solicitud de cita fue enviada. El consultorio la confirmará pronto por teléfono o correo.",
		})
	}
}

// findOrCreatePublicBookingPatient decide a qué paciente pertenece una
// solicitud pública, con dos niveles de coincidencia para evitar
// duplicados:
//
//  1. Primero busca entre los pacientes YA vinculados a este doctor, por
//     correo o por teléfono — es el caso más común de duplicidad: el
//     paciente ya está en la agenda de este consultorio pero escribió el
//     correo con mayúsculas distintas, o directamente puso otro correo
//     pero el mismo teléfono que ya tenía registrado.
//  2. Si no aparece ahí, busca globalmente por correo (puede existir por
//     otro doctor, o por otra solicitud pública anterior) y lo reutiliza.
//  3. Si tampoco existe, crea un paciente nuevo.
//
// El segundo booleano indica si el paciente ya estaba vinculado a este
// doctor (para no volver a intentar vincularlo).
func findOrCreatePublicBookingPatient(db *gorm.DB, doctor models.Doctor, req publicAppointmentRequest) (*models.Patient, bool, error) {
	email := strings.ToLower(strings.TrimSpace(req.PatientEmail))
	phone := strings.TrimSpace(req.PatientPhone)

	var patient models.Patient
	err := db.Joins("JOIN doctor_patients ON doctor_patients.patient_id = patients.id").
		Where("doctor_patients.doctor_id = ? AND (LOWER(patients.email) = ? OR patients.phone = ?)", doctor.ID, email, phone).
		First(&patient).Error
	if err == nil {
		return &patient, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	err = db.Where("LOWER(email) = ?", email).First(&patient).Error
	if err == nil {
		return &patient, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	patient = models.Patient{
		FirstName: req.PatientFirstName,
		LastName:  req.PatientLastName,
		Phone:     phone,
		Email:     email,
	}
	if err := db.Create(&patient).Error; err != nil {
		return nil, false, err
	}
	return &patient, false, nil
}

var slugNonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// generateDoctorSlug arma el identificador de la URL pública de un doctor
// (/dr/:slug) a partir de su nombre. Siempre incluye el ID al final: es la
// forma más simple de garantizar que nunca choque con el de otro doctor,
// sin tener que hacer una consulta extra a la base de datos para revisar
// colisiones.
func generateDoctorSlug(fullName string, doctorID uint) string {
	normalized := strings.ToLower(strings.TrimSpace(fullName))
	normalized = removeAccents(normalized)
	normalized = slugNonAlphanumeric.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		normalized = "doctor"
	}
	return normalized + "-" + strconv.FormatUint(uint64(doctorID), 10)
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ü", "u",
)

func removeAccents(s string) string {
	return accentReplacer.Replace(s)
}
