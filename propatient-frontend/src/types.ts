export interface MedicalHistory {
  allergies: string;
  pathological_history: string;
  non_pathological_history: string;
  surgical_history: string;
  current_medication?: string;
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
  documents?: any[];
  registrationStatus?: string;
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
  MedicalHistory?: MedicalHistory;
  Appointments?: Appointment[];
}

export interface ConsultationNotes {
  subjective: string;
  objective: string;
  diagnosis: string;
  treatmentPlan: string;
}