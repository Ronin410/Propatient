package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CurrentLegalNoticeVersion identifica el texto vigente de los Términos y
// Condiciones + Aviso de Privacidad (hoy tratados como un solo bloque de
// aceptación, ver Doctor.TermsAcceptedVersion y ConsentRecord.NoticeVersion).
// Se guarda junto con cada aceptación para poder acreditar exactamente qué
// versión aceptó cada quien; si el contenido de /terminos o /privacidad
// cambia de forma material, este valor debe subir para que las nuevas
// aceptaciones no se confundan con las anteriores.
const CurrentLegalNoticeVersion = "2026-07-21"

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
	// CedulaDocumentPath es la foto/PDF del documento de la cédula
	// profesional en sí (distinto de IneDocumentPath, que es solo la
	// identificación oficial) — le da al revisor de AdminPendingDoctors
	// algo concreto contra qué cotejar el número capturado en LicenseNumber.
	CedulaDocumentPath string `json:"cedulaDocumentPath"`
	Resume             string `json:"resume"`
	RecipeLegend       string `json:"recipeLegend"`
	AvatarUrl          string `json:"avatarUrl"`
	LogoUrl            string `json:"logoUrl"`
	// SignatureUrl es una imagen (idealmente PNG transparente) de la firma
	// manuscrita del doctor, subida una sola vez desde su perfil y
	// estampada en cada receta (ver buildRecipeDocDefinition en el
	// frontend) — no es una firma criptográfica, solo una representación
	// visual más fiel que el texto "FIRMA DEL MÉDICO" que se usa como
	// respaldo si no la ha configurado.
	SignatureUrl string `json:"signatureUrl"`

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

	// Clínica (alternativa a la suscripción individual de arriba): si
	// ClinicID no es nil, este doctor pertenece a una clínica y su acceso
	// se valida contra la suscripción de ESA clínica, no la propia (ver
	// billing.RequireActiveSubscription) — SubscriptionStatus/TrialEndsAt/
	// Stripe* de arriba dejan de importar mientras dure. IsClinicOwner
	// marca a quien la creó y a quien Stripe le cobra.
	ClinicID      *uint `gorm:"index" json:"clinicId"`
	IsClinicOwner bool  `gorm:"default:false" json:"isClinicOwner"`

	// Invitación pendiente a una clínica, antes de aceptarla. A diferencia
	// de Staff.InviteToken, aquí el doctor YA tiene cuenta y contraseña
	// propias — el token solo confirma que de verdad quiere unirse (y no
	// que alguien lo metió sin avisarle), ver handlers.AcceptClinicInvite.
	PendingClinicID            *uint      `json:"pendingClinicId"`
	ClinicInviteToken          string     `json:"-"`
	ClinicInviteTokenExpiresAt *time.Time `json:"-"`

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

	// Redes sociales / sitio propio: enlaces opcionales que se muestran en
	// el perfil público (ver toPublicDoctorSummary). Nunca se validan como
	// URLs "reales" contra un servicio externo — solo se muestran tal cual
	// si no vienen vacías.
	FacebookUrl  string `json:"facebookUrl"`
	InstagramUrl string `json:"instagramUrl"`
	LinkedinUrl  string `json:"linkedinUrl"`
	TwitterUrl   string `json:"twitterUrl"`
	TiktokUrl    string `json:"tiktokUrl"`
	YoutubeUrl   string `json:"youtubeUrl"`
	WebsiteUrl   string `json:"websiteUrl"`

	// Aceptación obligatoria de Términos y Condiciones + Aviso de
	// Privacidad: ningún doctor puede entrar al dashboard sin haberla
	// dado (ver OnboardingGuard.tsx en el frontend y
	// auth.AcceptTermsHandler). TermsAcceptedAt nil = no aceptados
	// todavía. Version/IP quedan como evidencia de qué texto exacto
	// aceptó y desde dónde, para el mismo tipo de trazabilidad que ya
	// usa internal/audit — IP no se expone al frontend, no aporta nada
	// ahí y es un dato personal en sí mismo.
	TermsAcceptedAt      *time.Time `json:"termsAcceptedAt"`
	TermsAcceptedVersion string     `json:"termsAcceptedVersion"`
	TermsAcceptedIP      string     `json:"-"`

	// Código propio, permanente, para invitar a otros doctores — solo se
	// muestra una vez que la suscripción está activa (ver
	// handlers.GetReferralCode). Se genera al crear la cuenta; las
	// cuentas creadas antes de este campo se rellenan en un paso de
	// compatibilidad en main.go, mismo patrón que la limpieza de
	// TrialEndsAt de ahí mismo. json:"-" porque se expone solo a través
	// de GetReferralCode, no en este payload ya grande.
	//
	// Índice normal, NO "uniqueIndex": mismo motivo que PublicSlug arriba
	// — todas las filas existentes arrancan en "" y un índice único
	// chocaría en cuanto haya dos. generateReferralCode ya garantiza
	// unicidad a nivel de aplicación (reintenta contra la tabla antes de
	// asignar), igual que generateDoctorSlug hace con PublicSlug.
	ReferralCode string `gorm:"index" json:"-"`

	// Contador propio para el folio de recetas (ver
	// handlers.GetOrAssignRecipeNumber y Appointment.RecipeNumber): cada
	// vez que este doctor genera una receta NUEVA se incrementa dentro de
	// una transacción con bloqueo de fila, para que dos recetas suyas
	// nunca terminen con el mismo folio aunque las genere casi al mismo
	// tiempo desde dos pestañas. json:"-" porque no aporta nada fuera de
	// ese cálculo — el folio que sí importa mostrar es el de la cita.
	LastRecipeNumber uint `gorm:"default:0" json:"-"`

	Patients []Patient `gorm:"many2many:doctor_patients;" json:"-"`
}

// DoctorGalleryImage es una foto adicional (consultorio, equipo, el propio
// doctor) para el perfil público — distinta del AvatarUrl/LogoUrl, que son
// una sola imagen fija cada uno. Sin límite de filas a nivel de base de
// datos; el límite de cuántas puede subir un doctor vive en el handler
// (ver internal/handlers/gallery_handler.go), no aquí.
type DoctorGalleryImage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	DoctorID  uint      `gorm:"not null;index" json:"doctorId"`
	ImagePath string    `json:"imagePath"`
}

// PushSubscription es la suscripción de notificaciones push (Web Push/
// VAPID) de un navegador/dispositivo concreto de un doctor — un doctor
// puede tener varias (celular, tablet, distintos navegadores), por eso es
// uno-a-muchos y no una columna en Doctor. Endpoint es único: volver a
// suscribirse desde el mismo navegador actualiza la fila en vez de
// duplicarla (ver handlers.SavePushSubscription). Se borra sola cuando
// el proveedor push responde 404/410 (la suscripción ya expiró del lado
// del navegador) — ver internal/webpush.SendToDoctor.
type PushSubscription struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	DoctorID  uint      `gorm:"not null;index" json:"doctorId"`
	Endpoint  string    `gorm:"not null;uniqueIndex" json:"endpoint"`
	P256dhKey string    `gorm:"not null" json:"-"`
	AuthKey   string    `gorm:"not null" json:"-"`
}

// Clinic agrupa a varios doctores bajo una sola suscripción de Stripe —
// alternativa a que cada uno pague la suya por separado. El dueño
// (OwnerDoctorID) es quien la creó y a quien Stripe le factura; los demás
// doctores se suman por invitación (ver handlers.InviteDoctorToClinic) y
// dejan de depender de su propia suscripción mientras pertenezcan aquí.
//
// El precio son DOS conceptos en la misma suscripción de Stripe: un monto
// fijo que cubre hasta billing.ClinicBaseIncludedDoctors doctores
// (StripeBaseItemID), y un cargo por cantidad por cada doctor que la
// sobrepase (StripeExtraItemID, vacío mientras la clínica tenga 5 o
// menos — se crea en Stripe apenas se suma el sexto, y se borra si vuelve
// a bajar a 5, ver billing.SetClinicExtraDoctorQuantity). Sin periodo de
// prueba: a diferencia de un doctor individual, el cobro arranca en
// cuanto se completa el Checkout.
type Clinic struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `json:"name"`

	OwnerDoctorID uint `gorm:"not null;index" json:"ownerDoctorId"`

	// "incomplete" (creada pero el Checkout aún no se completó) |
	// "active" | "past_due" | "canceled" | "trialing" (acceso gratuito
	// otorgado a mano por el superadmin, ver handlers.GrantClinicFreeAccess
	// — el plan de clínica no tiene prueba gratis automática, solo esta
	// manual) — mismos cinco estados que Doctor.SubscriptionStatus.
	SubscriptionStatus string `gorm:"default:'incomplete'" json:"subscriptionStatus"`

	// TrialEndsAt solo se usa junto con SubscriptionStatus "trialing" (ver
	// comentario arriba) — igual que Doctor.TrialEndsAt.
	TrialEndsAt *time.Time `json:"trialEndsAt"`

	// Ubicación única de la clínica — a diferencia de un doctor
	// independiente (que tiene su propia dirección editable), un
	// consultorio de clínica no es "movible" por cada doctor por separado:
	// solo el dueño la edita aquí (ver handlers.UpdateClinicLocation), y el
	// directorio público muestra ESTA dirección para todos sus doctores en
	// vez de la individual de cada quien (ver toPublicDoctorSummary).
	Address   string   `json:"address"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`

	StripeCustomerID     string `json:"-"`
	StripeSubscriptionID string `json:"-"`
	StripeBaseItemID     string `json:"-"`
	StripeExtraItemID    string `json:"-"`
}

// ClinicEmailInvite es una invitación a la clínica para un correo que
// TODAVÍA no tiene cuenta en ProPatient — a diferencia de
// Doctor.ClinicInviteToken (que vive en la fila del doctor ya
// registrado, con un token que confirma su consentimiento explícito),
// esta no tiene a qué doctor amarrarse todavía. Se resuelve sola, sin
// paso de aceptación, en cuanto ese correo se registra Y su cédula queda
// aprobada (ver handlers.ApproveDoctorCedula) — nunca antes, para no
// meter a la clínica a una cuenta sin verificar.
type ClinicEmailInvite struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	CreatedAt         time.Time `json:"createdAt"`
	ClinicID          uint      `gorm:"index;not null" json:"clinicId"`
	Email             string    `gorm:"index;not null" json:"email"`
	InvitedByDoctorID uint      `json:"invitedByDoctorId"`
	ExpiresAt         time.Time `json:"expiresAt"`
	// ConsumedAt queda nil mientras espera; se marca (nunca se borra la
	// fila, sirve de bitácora) en cuanto un doctor con ese correo se une.
	ConsumedAt *time.Time `json:"consumedAt"`
}

// Review es la reseña que un paciente deja de un doctor después de una
// consulta ya completada. La fila nace en cuanto la cita pasa a
// COMPLETED (ver UpdateAppointment), con Token pero sin calificación
// todavía — el paciente la completa desde el link que le llega por
// WhatsApp (ver review_handler.go), el mismo patrón de invitación de un
// solo uso que Staff.InviteToken. AppointmentID es único: una cita solo
// genera una invitación a reseña una vez, sin importar cuántas veces se
// vuelva a guardar ya completada.
//
// Approved empieza en false a propósito: una reseña recién enviada NO
// aparece de inmediato en el perfil público — el doctor la revisa y la
// aprueba primero, para evitar publicar sin filtro algo negativo, spam, o
// con datos de salud sensibles que el paciente haya escrito por error.
type Review struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	DoctorID      uint      `gorm:"not null;index" json:"doctorId"`
	PatientID     uint      `gorm:"not null" json:"patientId"`
	AppointmentID uint      `gorm:"not null;uniqueIndex" json:"appointmentId"`

	Rating  int    `json:"rating"` // 1-5, 0 mientras no se haya enviado
	Comment string `gorm:"type:text" json:"comment"`

	Approved bool `gorm:"default:false" json:"approved"`

	Token          string     `json:"-"`
	TokenExpiresAt *time.Time `json:"-"`
	SubmittedAt    *time.Time `json:"submittedAt"`
}

// Referral registra que un doctor se registró usando el código de
// invitación de otro (ver Doctor.ReferralCode). Nace en "pending" en
// cuanto el nuevo doctor captura un código válido al completar su
// perfil; pasa a "rewarded" solo cuando el PAGO del referido se
// completa de verdad (ver handlers.grantReferralRewardIfPending desde
// StripeWebhook) — nunca por el solo hecho de registrarse, para que no
// se puedan farmear semanas gratis con cuentas que jamás pagan.
//
// ReferredDoctorID con uniqueIndex (tabla nueva, sin filas viejas que
// choquen — a diferencia de Doctor.ReferralCode arriba): un doctor solo
// puede haber sido referido una vez, sin importar cuántas veces se
// intente aplicar un código distinto después.
type Referral struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	CreatedAt        time.Time  `json:"createdAt"`
	ReferrerDoctorID uint       `gorm:"not null;index" json:"referrerDoctorId"`
	ReferredDoctorID uint       `gorm:"not null;uniqueIndex" json:"referredDoctorId"`
	Status           string     `gorm:"default:'pending'" json:"status"` // "pending" | "rewarded"
	RewardedAt       *time.Time `json:"rewardedAt"`
}

// MaxReferralsPerDoctor es el tope de invitaciones que un mismo doctor
// puede tener en vuelo (pendientes + ya premiadas) — pasado este número,
// cualquier código nuevo que alguien intente usar contra él se rechaza
// en silencio, indistinguible de un código inválido (decisión explícita
// del producto: quien lo intenta no debe poder inferir si el código
// nunca existió o si ya se agotó).
const MaxReferralsPerDoctor = 10

// Staff representa a un miembro del personal (secretaria/asistente) — una
// sola identidad (correo/contraseña) que puede estar vinculada a VARIOS
// consultorios a la vez (ver DoctorStaff), para el caso de una clínica
// donde la misma persona administra la agenda de varios doctores. Su
// token de sesión lleva el doctorID del consultorio con el que entró en
// ese momento (para que toda la app siga filtrando por doctorID sin
// cambios), más el claim "role": "STAFF" que el middleware usa para negar
// el acceso a historial clínico, contenido de consultas y configuración
// del perfil/facturación del doctor (ver auth.RequireDoctorRole).
type Staff struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	FullName  string         `json:"fullName"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	// Nunca se envía al frontend.
	PasswordHash string `json:"-"`
	// PasswordSet distingue una invitación pendiente (aún sin contraseña)
	// de una cuenta ya activada.
	PasswordSet          bool       `gorm:"default:false" json:"passwordSet"`
	InviteToken          string     `json:"-"`
	InviteTokenExpiresAt *time.Time `json:"-"`
	// Token de un solo uso para "olvidé mi contraseña" (ver
	// handlers.RequestStaffPasswordReset/ResetStaffPassword), mismo patrón
	// que InviteToken pero con vigencia mucho más corta.
	PasswordResetToken          string     `json:"-"`
	PasswordResetTokenExpiresAt *time.Time `json:"-"`
}

// DoctorStaff vincula un Staff con UN consultorio. Reemplaza el antiguo
// Staff.DoctorID de uno-a-uno: la misma persona puede tener un vínculo
// activo con varios doctores (ej. una recepcionista compartida entre
// varios consultorios de una misma clínica), cada uno con su propio
// Active — desactivar el acceso desde un consultorio no debe afectar el
// acceso a los demás. Es una tabla de unión explícita (no el many2many
// implícito de GORM) porque necesita esta columna extra.
type DoctorStaff struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	DoctorID  uint      `gorm:"not null;index;uniqueIndex:idx_doctor_staff_pair" json:"doctorId"`
	StaffID   uint      `gorm:"not null;index;uniqueIndex:idx_doctor_staff_pair" json:"staffId"`
	Active    bool      `gorm:"default:true" json:"active"`
}

// TableName fija el nombre real de la tabla: el pluralizador de GORM
// convertiría "DoctorStaff" en "doctor_staffs", pero el resto del código
// (handlers, migración de compatibilidad en main.go) usa "doctor_staff" en
// SQL crudo, así que se fija explícitamente para que coincida.
func (DoctorStaff) TableName() string {
	return "doctor_staff"
}

// SuperAdmin es una cuenta interna de ProPatient (no de un consultorio),
// separada por completo de Doctor/Staff: su JWT nunca lleva "userId" y no
// puede tocar ninguna ruta de /api gated por doctor. Hoy solo se usa para
// revisar y aprobar/rechazar la cédula profesional que un doctor sube
// durante el onboarding (ver internal/handlers/admin_handler.go), en vez
// de aprobarla a mano con SQL directo.
type SuperAdmin struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Username     string    `gorm:"unique;not null" json:"username"`
	PasswordHash string    `json:"-"`
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
	// El teléfono/correo de un paciente menor de edad pertenecen a quien
	// ejerce su patria potestad o tutela, no al propio paciente — esta
	// bandera lo deja explícito en el expediente (para el doctor) y en el
	// registro de quién dio el consentimiento (ver CreatePublicAppointment,
	// donde el checkbox de aceptación cambia de texto declarando ser
	// padre/madre/tutor cuando se marca).
	IsMinor bool `gorm:"default:false" json:"isMinor"`
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
	// Quién originó la cita: "DOCTOR" (el doctor/personal la agendó
	// directo) o "PUBLIC" (el paciente la solicitó desde el directorio
	// público, ver CreatePublicAppointment). Se fija una sola vez al
	// crearla y nunca cambia — a diferencia de Status, sigue siendo
	// consultable después de que una solicitud pública se confirma (ahí
	// Status pasa a "PENDING" igual que una cita creada por el doctor, y
	// se perdería la distinción sin este campo). "" en citas creadas
	// antes de este campo (dato desconocido, no se adivina).
	Source    string `json:"source"`
	Diagnosis string `gorm:"type:text" json:"diagnosis"`
	// Código CIE-10 elegido en el buscador (ver Cie10Code), opcional —
	// Diagnosis sigue siendo el texto libre que se ve/imprime; este campo
	// es la versión estructurada para catálogo/interoperabilidad futura
	// (HL7), no reemplaza al texto. "" si el doctor no usó el buscador.
	DiagnosisCode      string         `json:"diagnosisCode"`
	TreatmentPlan      string         `gorm:"type:text" json:"treatmentPlan"`
	DynamicNotes       datatypes.JSON `json:"dynamic_notes" gorm:"type:jsonb"`
	Notes              string         `gorm:"type:text" json:"notes"`
	RegistrationStatus string         `gorm:"default:'REGISTERED'" json:"registrationStatus"`
	RecipePDFPath      string         `json:"recipePdfPath"`
	// Folio único de la receta de ESTA cita, tomado del contador propio del
	// doctor (Doctor.LastRecipeNumber) la primera vez que genera la receta
	// — ver handlers.GetOrAssignRecipeNumber. Puntero: nil mientras nunca
	// se ha generado ninguna receta para esta cita. Se queda fijo aunque
	// el doctor vuelva a generar/imprimir la misma receta después (sigue
	// siendo la misma receta, no una nueva), así ningún folio se repite
	// entre dos recetas de verdad distintas del mismo doctor.
	RecipeNumber     *uint             `json:"recipeNumber"`
	MedicalDocuments []MedicalDocument `gorm:"foreignKey:AppointmentID" json:"documents"`
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

	// Token opaco para el link/QR de "sube tus documentos antes de la
	// cita" (ver ConsultationManager → toggleQR y handlers.GetAppointmentUploadLink/
	// GetPublicUploadInfo/PublicUploadDocuments). Se genera la primera vez
	// que el doctor pide el link, no antes — "" significa que nunca se
	// generó. A diferencia de Review.Token, no se consume de un solo uso:
	// el paciente puede volver a escanear el mismo QR las veces que
	// necesite antes de que expire.
	UploadToken          string     `gorm:"index" json:"-"`
	UploadTokenExpiresAt *time.Time `json:"-"`
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

// DoctorSchedule guarda el horario laboral configurable del consultorio:
// qué días atiende, de qué hora a qué hora, y descansos que excluir dentro
// de esa ventana (ej. "de 2 a 3 no trabajo"). Lo puede editar tanto el
// doctor como su personal (ver internal/handlers/schedule_handler.go, sin
// RequireDoctorRole) — a diferencia de la plantilla de notas o el perfil,
// que son solo del doctor. "Days" guarda un objeto JSON con una entrada
// por día de la semana (ver weekSchedule en schedule_handler.go). Si un
// doctor nunca configura su horario, no existe fila y no hay ninguna
// restricción al agendar — mismo criterio de "opcional, no rompe nada
// existente" que el resto de las integraciones de esta app.
type DoctorSchedule struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DoctorID  uint           `json:"doctorId" gorm:"uniqueIndex;not null"`
	Days      datatypes.JSON `json:"days" gorm:"type:jsonb;not null"`
}

// AuditLog es la pista de auditoría del consultorio: quién accedió o
// modificó qué dato clínico, cuándo y desde qué IP (ver internal/audit).
// Append-only por diseño — ningún handler debe actualizar ni borrar una
// fila ya escrita aquí, solo internal/audit.Log crea filas nuevas.
// DoctorID identifica siempre al CONSULTORIO dueño del dato afectado
// (para que el personal comparta la misma bitácora que su doctor), no a
// quién hizo la acción — eso lo dicen ActorRole/ActorID/ActorName.
// PatientID es opcional (nil en acciones que no son de un paciente en
// particular, ej. login) — cuando aplica, permite consultar de un
// vistazo toda la bitácora de un expediente sin importar si la entidad
// tocada fue el propio paciente, su historial, una cita o un documento.
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
	DoctorID   uint      `gorm:"index;not null" json:"doctorId"`
	PatientID  *uint     `gorm:"index" json:"patientId"`
	ActorRole  string    `json:"actorRole"` // "MEDICO" o "STAFF"
	ActorID    uint      `json:"actorId"`
	ActorName  string    `json:"actorName"` // snapshot: sigue siendo legible aunque la cuenta se borre después
	Action     string    `json:"action"`    // "created" | "updated" | "viewed" | "deleted"
	EntityType string    `json:"entityType"`
	EntityID   uint      `json:"entityId"`
	Details    string    `json:"details"`
	IPAddress  string    `json:"ipAddress"`
}

// AppointmentNoteHistory preserva, de forma inmutable, el contenido
// clínico de una cita justo ANTES de cada edición — así ninguna nota
// firmada se pierde al corregirla, aunque la vista "actual" de la cita
// (Appointment.Diagnosis/TreatmentPlan/Notes/DynamicNotes) siga siendo un
// solo valor editable en vez de una lista de adendas. Ver
// handlers.UpdateAppointment, que inserta una fila aquí ANTES de
// sobreescribir, solo si el contenido clínico de verdad cambió.
// Append-only: ningún handler debe actualizar ni borrar una fila ya
// escrita.
type AppointmentNoteHistory struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time `gorm:"index" json:"createdAt"`
	AppointmentID uint      `gorm:"index;not null" json:"appointmentId"`

	// Snapshot del contenido clínico tal como estaba antes de este cambio.
	PreviousDiagnosis     string `gorm:"type:text" json:"previousDiagnosis"`
	PreviousDiagnosisCode string `json:"previousDiagnosisCode"`
	PreviousTreatmentPlan string `gorm:"type:text" json:"previousTreatmentPlan"`
	PreviousNotes         string `gorm:"type:text" json:"previousNotes"`
	PreviousDynamicNotes  string `gorm:"type:text" json:"previousDynamicNotes"`

	ChangedByRole string `json:"changedByRole"`
	ChangedByID   uint   `json:"changedById"`
	ChangedByName string `json:"changedByName"`
	IPAddress     string `json:"ipAddress"`
}

// ConsentRecord es la evidencia de que un paciente (o su padre/madre/tutor,
// ver IsMinor) aceptó el tratamiento de sus datos de salud al agendar una
// cita pública sin cuenta (ver handlers.CreatePublicAppointment). El
// checkbox de consentimiento ya era obligatorio para poder enviar el
// formulario, pero antes esa aceptación no quedaba registrada en ningún
// lado — solo el hecho de que la cita existiera. Con este modelo queda
// quién aceptó (vía el paciente/cita creados), cuándo, desde qué IP y qué
// versión del Aviso de Privacidad aceptó, para poder acreditarlo ante una
// eventual revisión (LFPDPPP). Append-only, igual que AuditLog/
// AppointmentNoteHistory: nunca se actualiza ni se borra una fila ya
// escrita aquí.
type ConsentRecord struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time `gorm:"index" json:"createdAt"`
	PatientID     uint      `gorm:"index;not null" json:"patientId"`
	AppointmentID uint      `gorm:"index;not null" json:"appointmentId"`
	DoctorID      uint      `gorm:"index;not null" json:"doctorId"`
	// true si quien aceptó lo hizo declarando ser padre/madre/tutor de un
	// paciente menor de edad (ver publicAppointmentRequest.IsMinorPatient).
	AcceptedAsGuardian bool   `json:"acceptedAsGuardian"`
	NoticeVersion      string `json:"noticeVersion"`
	IPAddress          string `json:"ipAddress"`
}

// Cie10Code es el catálogo oficial de diagnósticos CIE-10 (DGIS/CENETEC,
// actualización abril 2024), cargado una sola vez desde un CSV embebido en
// el binario (ver internal/database.SeedCie10Catalog) — nunca se edita
// desde la app, es de solo lectura para el buscador de diagnóstico. Solo
// incluye códigos vigentes (VALID="SI" en el catálogo original): las
// categorías de 3 caracteres sin subcategoría específica quedan fuera a
// propósito, son demasiado genéricas para capturarse como diagnóstico
// real (ver el comentario de importación en internal/database/seed.go).
type Cie10Code struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	Code           string `gorm:"uniqueIndex;not null" json:"code"` // ej. "J449" (equivale a J44.9)
	Name           string `gorm:"index;not null" json:"name"`
	ChapterKey     string `json:"chapterKey"`
	Chapter        string `json:"chapter"`
	SexRestriction string `json:"sexRestriction"` // "H", "M", o "" sin restricción
}

// WhatsAppMessage guarda tanto los mensajes ENTRANTES que llegan al número
// de WhatsApp de ProPatient (paciente respondiendo un aviso, o doctor
// pidiendo soporte) como los OUTBOUND que el superadmin manda de vuelta
// desde la bandeja — nunca los avisos automáticos normales (confirmación,
// recordatorio, etc.), esos siguen su propio camino en internal/whatsapp
// sin pasar por aquí. El propio número de WhatsApp es compartido por todos
// los doctores, así que la única forma de saber a quién pertenece un
// mensaje entrante es por teléfono (ver handlers.classifyInboundWhatsApp):
// Category distingue si el remitente es un doctor pidiendo soporte o
// alguien más (paciente conocido o número desconocido) — ambos casos caen
// en bandejas separadas que solo ve el superadmin, nunca el doctor dueño
// de la cita, para no convertir esto en un canal de soporte por doctor.
// DoctorID/PatientID son solo un best-effort de contexto para mostrarle al
// superadmin con quién podría estar relacionado el mensaje (ver el
// comentario largo en el handler del webhook); no controlan quién puede
// verlo, eso siempre es "solo superadmin".
type WhatsAppMessage struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time  `gorm:"index" json:"createdAt"`
	Direction string     `gorm:"not null" json:"direction"`      // "INBOUND" | "OUTBOUND"
	Category  string     `gorm:"index;not null" json:"category"` // "PATIENT" | "DOCTOR_SUPPORT"
	Phone     string     `gorm:"index;not null" json:"phone"`    // E.164, la otra parte de la conversación (nunca el número de ProPatient)
	Body      string     `gorm:"not null" json:"body"`
	TwilioSid string     `gorm:"uniqueIndex" json:"-"` // idempotencia: Twilio puede reintentar el mismo webhook
	DoctorID  *uint      `gorm:"index" json:"doctorId"`
	PatientID *uint      `gorm:"index" json:"patientId"`
	ReadAt    *time.Time `json:"readAt"`
}

// TableName fija el nombre de la tabla explícitamente — el conversor
// automático de GORM lo hubiera dejado como "whats_app_messages" (parte
// cada mayúscula como si fuera una palabra nueva), que no es el nombre que
// nadie esperaría al escribir SQL a mano contra esta tabla.
func (WhatsAppMessage) TableName() string {
	return "whatsapp_messages"
}
