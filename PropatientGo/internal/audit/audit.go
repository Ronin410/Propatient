// Package audit registra la pista de auditoría del consultorio (quién
// accedió o modificó un dato clínico, cuándo y desde qué IP) — ver
// models.AuditLog. A diferencia de whatsapp/storage/billing, esto NO es
// una integración opcional: siempre está activo, sin variables de entorno
// que lo enciendan o apaguen, porque la propia bitácora es el requisito
// de cumplimiento (NOM-024) que justifica su existencia.
package audit

import (
	"log"

	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Acciones reconocidas — mantener esta lista corta y consistente entre
// handlers hace que la bitácora sea filtrable/legible en vez de texto libre.
const (
	ActionCreated = "created"
	ActionUpdated = "updated"
	ActionViewed  = "viewed"
	ActionDeleted = "deleted"
)

// Tipos de entidad reconocidos.
const (
	EntityPatient         = "patient"
	EntityMedicalHistory  = "medical_history"
	EntityAppointment     = "appointment"
	EntityMedicalDocument = "medical_document"
)

// CurrentActor identifica quién hace la petición actual: el doctor dueño
// del consultorio, o la cuenta de personal (Staff) que inició sesión con
// su token. Exportada porque también la usa el guardado de versiones
// inmutables de notas clínicas (ver handlers.UpdateAppointment), no solo
// Log — evita duplicar esta misma búsqueda en dos lugares.
func CurrentActor(db *gorm.DB, c *gin.Context, doctorID uint) (role string, actorID uint, actorName string) {
	roleVal, _ := c.Get("role")
	role, _ = roleVal.(string)
	if role == "" {
		role = "MEDICO"
	}

	actorID = doctorID

	if role == "STAFF" {
		if staffIDVal, ok := c.Get("staffId"); ok {
			if sid, ok := staffIDVal.(uint); ok {
				actorID = sid
				var staff models.Staff
				if err := db.Select("full_name").First(&staff, sid).Error; err == nil {
					actorName = staff.FullName
				}
			}
		}
	} else {
		var doctor models.Doctor
		if err := db.Select("full_name").First(&doctor, doctorID).Error; err == nil {
			actorName = doctor.FullName
		}
	}

	return role, actorID, actorName
}

// Log inserta una fila en la bitácora. Es "mejor esfuerzo": un error al
// escribir la auditoría nunca debe tumbar la petición real del usuario
// (igual que el resto de integraciones de la app) — solo se registra en
// el log del servidor. doctorID es siempre el consultorio dueño del dato
// afectado (para que el personal comparta la bitácora de su doctor);
// patientID es nil si la entidad no pertenece a un paciente en particular.
func Log(db *gorm.DB, c *gin.Context, doctorID uint, patientID *uint, action, entityType string, entityID uint, details string) {
	role, actorID, actorName := CurrentActor(db, c, doctorID)

	entry := models.AuditLog{
		DoctorID:   doctorID,
		PatientID:  patientID,
		ActorRole:  role,
		ActorID:    actorID,
		ActorName:  actorName,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Details:    details,
		IPAddress:  c.ClientIP(),
	}
	if err := db.Create(&entry).Error; err != nil {
		log.Printf("⚠️ No se pudo guardar bitácora de auditoría (%s %s#%d, doctor %d): %v", action, entityType, entityID, doctorID, err)
	}
}
