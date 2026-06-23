package models

import (
	"time"

	"gorm.io/gorm"
)

// Doctor representa la entidad DOCTOR_USER (Tu Doctor.java)
type Doctor struct {
	gorm.Model
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
	Patients         []Patient      `gorm:"many2many:doctor_patients;" json:"-"`
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
	Appointments   []Appointment   `json:"-" gorm:"foreignKey:PatientID"`       // Ocultar para evitar ciclos infinitos en JSON
	Doctors        []Doctor        `gorm:"many2many:doctor_patients;" json:"-"` // Ocultamos la relación en JSON para evitar ciclos
}

// MedicalHistory representa tu MedicalHistory.java
type MedicalHistory struct {
	ID                   uint           `gorm:"primaryKey" json:"id"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
	PatientID            uint           `gorm:"unique" json:"patientId"`       // Un historial por paciente
	Patient              *Patient       `gorm:"foreignKey:PatientID" json:"-"` // Ocultamos la relación en JSON
	AllergiesDescription string         `gorm:"type:text" json:"allergiesDescription"`
	ChronicDiseaseDesc   string         `gorm:"type:text" json:"chronicDiseaseDescription"`
	FamilyHistory        string         `gorm:"type:text" json:"familyHistory"`
}

// Appointment representa tu Appointment.java
type Appointment struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	PatientID           uint           `json:"patientId"`
	Patient             *Patient       `gorm:"foreignKey:PatientID" json:"patient"` // Volvemos al estándar 'patient'
	DoctorID            uint           `json:"doctorId"`
	AppointmentDateTime time.Time      `json:"appointmentDateTime"` // Asegurar que coincida con el .ts de Angular
	Reason              string         `json:"reason"`
	Status              string         `gorm:"default:'PENDING'" json:"status"`
	Diagnosis           string         `gorm:"type:text" json:"diagnosis"`
	TreatmentPlan       string         `gorm:"type:text" json:"treatmentPlan"`
	Notes               string         `gorm:"type:text" json:"notes"`
	RegistrationStatus  string         `gorm:"default:'REGISTERED'" json:"registrationStatus"`
}

type MedicalDocument struct {
	FileName      string    `json:"filename"`
	FileType      string    `json:"fileType"`
	Data          []byte    `json:"data"`
	AppointmentID uint      `json:"appointmentId"`
	Prescription  bool      `json:"prescription"`
	UpdateAt      time.Time `json:"createdAt"`
	CreateAt      time.Time `json:"updatedAt"`
	DeleteAt      time.Time `json:"deletedAt"`
}

type GoogleTokenClaims struct {
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}
