export interface PublicDoctor {
  id: number;
  fullName: string;
  medicalSpecialty: string;
  publicBio: string;
  avatarUrl?: string;
  address: string;
  phone: string;
  latitude: number | null;
  longitude: number | null;
  publicSlug: string;
  // Horario laboral configurado (ver WeekSchedule) — null si el doctor
  // nunca lo configuró, en cuyo caso no hay ninguna restricción al agendar.
  schedule?: WeekSchedule | null;
  // Redes sociales / sitio propio — solo vienen si el doctor las llenó.
  facebookUrl?: string;
  instagramUrl?: string;
  linkedinUrl?: string;
  twitterUrl?: string;
  tiktokUrl?: string;
  youtubeUrl?: string;
  websiteUrl?: string;
  // Galería de fotos y reseñas aprobadas — solo vienen en el perfil
  // individual (/dr/:slug), no en el listado del directorio.
  galleryImages?: GalleryImage[];
  reviews?: PublicReview[];
  reviewsAverage?: number;
}

// Una foto de la galería del perfil público del doctor.
export interface GalleryImage {
  id: number;
  imagePath: string;
  created_at: string;
}

// Una reseña ya aprobada, tal como la ve un visitante del perfil público
// (sin datos del paciente más allá de su primer nombre).
export interface PublicReview {
  patientFirstName: string;
  rating: number;
  comment: string;
  submittedAt: string;
}

// Una reseña tal como la ve el doctor en su panel de gestión — incluye el
// estado de aprobación y el nombre completo del paciente.
export interface Review {
  id: number;
  doctorId: number;
  patientId: number;
  appointmentId: number;
  rating: number;
  comment: string;
  approved: boolean;
  submittedAt: string | null;
  patientFirstName: string;
  patientLastName: string;
}

// Un bloque de horas en formato "HH:MM" (24h) — la ventana laboral de un
// día, o un descanso dentro de ella (ver DayHours).
export interface TimeRange {
  start: string;
  end: string;
}

// El horario de un solo día de la semana: si el doctor atiende ese día,
// su ventana laboral, y los descansos a excluir dentro de ella (ej. "de 2
// a 3 no trabajo"). Espejo exacto de dayHours en el backend
// (internal/handlers/schedule_handler.go).
export interface DayHours {
  enabled: boolean;
  start: string;
  end: string;
  breaks: TimeRange[];
}

// El horario completo del consultorio, un DayHours por cada día de la
// semana — así se guarda y se agenda (ver DoctorSchedule en el backend).
export interface WeekSchedule {
  sunday: DayHours;
  monday: DayHours;
  tuesday: DayHours;
  wednesday: DayHours;
  thursday: DayHours;
  friday: DayHours;
  saturday: DayHours;
}

export interface Staff {
  id: number;
  doctorId: number;
  fullName: string;
  email: string;
  active: boolean;
  passwordSet: boolean;
  created_at?: string;
}

// Un consultorio al que una cuenta de personal tiene acceso activo, tal
// como lo devuelve StaffLoginHandler cuando hay más de uno (ver
// StaffLogin.tsx: pantalla "¿Con cuál consultorio quieres entrar?").
export interface StaffDoctorOption {
  doctorId: number;
  doctorName: string;
}

export interface MedicalHistory {
  allergies: string;
  pathological_history: string;
  non_pathological_history: string;
  surgical_history: string;
  current_medication?: string;
  hereditaryHistory?: string;
  gynecoObstetric?: string;
  habitsLifestyle?: string;
}

export interface MedicalDocument {
  id: number;
  filename: string;
  fileType: string;
  file_path: string;
  appointmentId: number;
  prescription: boolean;
}

export interface Appointment {
  id: number;
  appointmentDateTime: string;
  reason: string;
  status: string;
  patientId?: number;
  observations?: string;
  diagnosis?: string;
  treatmentPlan?: string;
  patient?: Patient;
  Patient?: Patient; // GORM preload default
  documents?: MedicalDocument[];
  registrationStatus?: string;
  recipePdfPath?: string;
  followUpDate?: string;
}

export interface Patient {
  id: number;
  firstName: string;
  FirstName?: string; // Go default
  lastName: string;
  LastName?: string;  // Go default
  email: string;
  phone: string;
  birthDate: string;
  gender: string;
  // El teléfono/correo de un paciente menor de edad pertenecen a su
  // padre/madre o tutor, no al propio paciente — ver PublicDoctorProfile.tsx
  // (checkbox de agendado público) y PatientForm.tsx.
  isMinor?: boolean;
  address?: string;
  updated_at?: string;
  medicalHistory?: MedicalHistory;
  appointments?: Appointment[];
  // El backend nunca envía estas variantes en mayúscula (json tags ya son
  // camelCase); se mantienen solo porque ConsultationManager.tsx aún las
  // referencia como fallback defensivo.
  MedicalHistory?: MedicalHistory;
  Appointments?: Appointment[];
}

export interface ConsultationNotes {
  subjective: string;
  objective: string;
  diagnosis: string;
  treatmentPlan: string;
}