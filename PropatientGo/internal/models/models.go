package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Doctor representa la entidad DOCTOR_USER (Tu Doctor.java)
type Doctor struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
	Username         string         `gorm:"unique;not null"`
	PasswordHash     string         `json:"-"` // Importante: El hash nunca se envía al frontend
	FullName         string         `json:"fullName"`
	MedicalSpecialty string         `json:"medicalSpecialty"`
	Email            string         `gorm:"unique" json:"email"`
	Phone            string         `json:"phone"`
	LicenseNumber    string         `json:"licenseNumber"`
	BirthDate        *time.Time     `json:"birthDate"` // Puntero para soportar nulos iniciales antes del onboarding
	Address          string         `json:"address"`
	PostalCode       string         `json:"postalCode"`
	RFC              string         `json:"rfc"`
	CURP             string         `json:"curp"`
	University       string         `json:"university"`
	ProfileCompleted bool           `gorm:"default:false" json:"profileCompleted"`
	CedulaValidated  string         `gorm:"type:varchar(20);default:'PENDIENTE'" json:"cedulaValidated"`
	IneDocumentPath  string         `json:"ineDocumentPath"`
	Resume           string         `json:"resume"`
	RecipeLegend     string         `json:"recipeLegend"`
	AvatarUrl        string         `json:"avatarUrl"`
	LogoUrl          string         `json:"logoUrl"`

	Patients []Patient `gorm:"many2many:doctor_patients;" json:"-"`
}

// Patient representa la entidad Patient.java
type Patient struct {
	ID             uint            `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      gorm.DeletedAt  `gorm:"index" json:"-"`
	FirstName      string          `json:"firstName"`
	LastName       string          `json:"lastName"`
	BirthDate      string          `json:"birthDate"`
	Gender         string          `json:"gender"`
	Email          string          `gorm:"unique" json:"email"`
	Phone          string          `json:"phone"`
	MedicalHistory *MedicalHistory `json:"medicalHistory,omitempty"`
	Appointments   []Appointment   `json:"appointments,omitempty" gorm:"foreignKey:PatientID"`
	Doctors        []Doctor        `gorm:"many2many:doctor_patients;" json:"-"` // Ocultamos la relación en JSON para evitar ciclos
}

// MedicalHistory representa tu MedicalHistory.java
type MedicalHistory struct {
	ID                     uint     `gorm:"primaryKey" json:"id"`
	PatientID              uint     `gorm:"unique" json:"patientId"`       // Un historial por paciente
	Patient                *Patient `gorm:"foreignKey:PatientID" json:"-"` // Ocultamos la relación en JSON
	Allergies              string   `json:"allergies"`
	PathologicalHistory    string   `json:"pathological_history"`
	NonPathologicalHistory string   `json:"non_pathological_history"`
	SurgicalHistory        string   `json:"surgical_history"`
	CurrentMedication      string   `json:"current_medication"`
	HereditaryHistory      string   `json:"hereditaryHistory"`
	GynecoObstetric        string   `json:"gynecoObstetric"`
	HabitsLifestyle        string   `json:"habitsLifestyle"`
}

// Appointment representa tu Appointment.java
type Appointment struct {
	ID                  uint              `gorm:"primaryKey" json:"id"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	DeletedAt           gorm.DeletedAt    `gorm:"index" json:"-"`
	PatientID           uint              `json:"patientId"`
	Patient             *Patient          `gorm:"foreignKey:PatientID" json:"patient"` // Volvemos al estándar 'patient'
	DoctorID            uint              `json:"doctorId"`
	AppointmentDateTime time.Time         `json:"appointmentDateTime"` // Asegurar que coincida con el .ts de Angular
	Reason              string            `json:"reason"`
	Status              string            `gorm:"default:'PENDING'" json:"status"`
	Diagnosis           string            `gorm:"type:text" json:"diagnosis"`
	TreatmentPlan       string            `gorm:"type:text" json:"treatmentPlan"`
	DynamicNotes        datatypes.JSON    `json:"dynamic_notes" gorm:"type:jsonb"`
	Notes               string            `gorm:"type:text" json:"notes"`
	RegistrationStatus  string            `gorm:"default:'REGISTERED'" json:"registrationStatus"`
	RecipePDFPath       string            `json:"recipePdfPath"`
	MedicalDocuments    []MedicalDocument `gorm:"foreignKey:AppointmentID" json:"documents"`
}

type MedicalDocument struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	FileName      string `json:"filename"`
	FileType      string `json:"fileType"`
	FilePath      string `json:"file_path"`
	AppointmentID uint   `json:"appointmentId"`
	Prescription  bool   `json:"prescription"`
}

type GoogleTokenClaims struct {
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Audience      string `json:"aud"`
}

type UpdateDocumentInput struct {
	Filename string `json:"filename" binding:"required"`
}

type DoctorTemplate struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	DoctorID  uint           `json:"doctorId" gorm:"uniqueIndex;not null"`
	Fields    datatypes.JSON `json:"fields" gorm:"type:jsonb;not null"` // Guarda el array de apartados configurados
}
