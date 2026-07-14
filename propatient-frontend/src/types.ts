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