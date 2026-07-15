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

	// Integración con Google Calendar (OAuth de servidor, no el login).
	// El refresh token nunca se envía al frontend; solo se expone si está
	// presente o no vía el campo calculado "googleCalendarConnected" en el
	// handler de perfil.
	GoogleCalendarRefreshToken string `json:"-"`

	// Suscripción (Stripe). SubscriptionStatus: "trialing" | "active" |
	// "past_due" | "canceled". Todo doctor nuevo arranca en "trialing" con
	// TrialEndsAt = fecha de alta + 14 días (ver auth.newTrialDoctor). El
	// middleware RequireActiveSubscription bloquea el resto de la API una
	// vez que ni la prueba ni una suscripción activa cubren al doctor.
	SubscriptionStatus   string     `gorm:"default:'trialing'" json:"subscriptionStatus"`
	TrialEndsAt          *time.Time `json:"trialEndsAt"`
	StripeCustomerID     string     `json:"-"`
	StripeSubscriptionID string     `json:"-"`

	// Directorio público (landing page): el doctor decide si aparece
	// (opt-in, PublicListed empieza en false). PublicSlug identifica su
	// URL pública (/dr/:slug), Latitude/Longitude se calculan a partir de
	// Address vía geocoding.GeocodeAddress cuando activa el listado o
	// cambia su dirección — ver UpdateCurrentDoctor.
	//
	// Índice normal, NO "uniqueIndex": generateDoctorSlug ya garantiza
	// unicidad incluyendo el ID del doctor, y un índice único sobre un
	// campo que empieza vacío ("") en todos los doctores existentes
	// choca en cuanto hay dos — el mismo bug real que ya se corrigió una
	// vez con Patient.Email (ver la limpieza de migración en main.go).
	PublicListed bool     `gorm:"default:false" json:"publicListed"`
	PublicBio    string   `json:"publicBio"`
	PublicSlug   string   `gorm:"index" json:"publicSlug"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`

	Patients []Patient `gorm:"many2many:doctor_patients;" json:"-"`
}

// Staff representa a un miembro del personal (secretaria/asistente) de un
// consultorio. Su token de sesión lleva el doctorID del dueño del
// consultorio (para que toda la app siga filtrando por doctorID sin
// cambios), más el claim "role": "STAFF" que el middleware usa para negar
// el acceso a historial clínico, contenido de consultas y configuración
// del perfil/facturación del doctor (ver auth.RequireDoctorRole).
type Staff struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	DoctorID  uint           `gorm:"not null;index" json:"doctorId"`
	FullName  string         `json:"fullName"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	// Nunca se envía al frontend.
	PasswordHash string `json:"-"`
	Active       bool   `gorm:"default:true" json:"active"`
	// PasswordSet distingue una invitación pendiente (aún sin contraseña)
	// de una cuenta ya activada.
	PasswordSet          bool       `gorm:"default:false" json:"passwordSet"`
	InviteToken          string     `json:"-"`
	InviteTokenExpiresAt *time.Time `json:"-"`
}

// Patient representa la entidad Patient.java
type Patient struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	FirstName string         `json:"firstName"`
	LastName  string         `json:"lastName"`
	BirthDate string         `json:"birthDate"`
	Gender    string         `json:"gender"`
	// Sin "unique" a nivel de columna: un mismo paciente puede estar
	// vinculado a varios doctores (many2many vía doctor_patients), así que
	// la unicidad de correo se valida en el handler (CreatePatient), no en
	// la base de datos. Con "unique" aquí, dos pacientes sin correo (campo
	// opcional) de dos doctores distintos colisionaban en el mismo valor
	// "" y la creación fallaba con un error de base de datos sin sentido.
	Email          string          `json:"email"`
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
	// Fecha sugerida de cita de control, marcada opcionalmente al finalizar
	// la consulta. Puntero para poder distinguir "sin seguimiento" (nil) de
	// una fecha real, y para poder limpiarla mandando JSON null.
	FollowUpDate *time.Time `gorm:"index" json:"followUpDate"`

	// ID del evento espejo en Google Calendar (si el doctor tiene la
	// integración conectada). Uso interno para poder actualizarlo/borrarlo
	// al reprogramar o cancelar la cita; nunca se expone al frontend.
	GoogleEventID string `json:"-"`

	// Marca cuándo se mandó el correo/WhatsApp de recordatorio al PACIENTE
	// (~24h antes de la cita); nil = todavía no se envía. Ver
	// workers.SendDueAppointmentReminders — evita reenviarlo en cada pasada
	// del worker en segundo plano.
	ReminderSentAt *time.Time `json:"-"`

	// Igual que ReminderSentAt, pero para el aviso al DOCTOR poco antes de
	// que empiece la cita (WhatsApp, ~60 min antes). Ver
	// workers.SendDueDoctorReminders.
	DoctorReminderSentAt *time.Time `json:"-"`
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
