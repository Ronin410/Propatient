import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Appointment } from '../types';
import api from '../api/axios';
import { getErrorMessage } from '../utils/errorMessage';
import { generateAndSaveRecipePDF, pdfMake } from '../utils/recipePdf';
import { toAbsoluteFileUrl } from '../utils/fileUrl';

export interface AppointmentFile {
  id?: number;
  name: string;
  type: string;
  size: number;
  url: string;
  originalFile?: File;
  isServerFile?: boolean;
}

export interface NoteSectionConfig {
  id: string;
  label: string;
  placeholder: string;
  required: boolean;
}

const DEFAULT_SECTIONS: NoteSectionConfig[] = [
  { id: 'subjetivo', label: 'Padecimiento Actual (Subjetivo)', placeholder: 'Lo que el paciente refiere...', required: false },
  { id: 'diagnostico', label: 'Diagnóstico', placeholder: 'Impresión diagnóstica...', required: true },
  { id: 'tratamiento', label: 'Plan y Tratamiento', placeholder: 'Receta e indicaciones...', required: true },
];

interface PatientFormMedicalHistory {
  allergies: string;
  pathological_history: string;
  non_pathological_history: string;
  surgical_history: string;
  current_medication: string;
  hereditaryHistory: string;
  gynecoObstetric: string;
  habitsLifestyle: string;
  additional_notes: string;
}

export interface PatientFormState {
  firstName: string;
  lastName: string;
  birthDate: string;
  gender: string;
  phone: string;
  email: string;
  address: string;
  medicalHistory: PatientFormMedicalHistory;
}

const EMPTY_PATIENT_FORM: PatientFormState = {
  firstName: '',
  lastName: '',
  birthDate: '',
  gender: '',
  phone: '',
  email: '',
  address: '',
  medicalHistory: {
    allergies: '',
    pathological_history: '',
    non_pathological_history: '',
    surgical_history: '',
    current_medication: '',
    hereditaryHistory: '',
    gynecoObstetric: '',
    habitsLifestyle: '',
    additional_notes: '',
  },
};

// Toda la lógica de datos/lifecycle de la pantalla de consulta: carga de la
// cita, formulario de paciente/antecedentes, notas dinámicas, archivos
// adjuntos, autoguardado + restauración de borrador local, bloqueo de
// navegación con cambios sin guardar, y generación/guardado de la receta.
export function useConsultation(appointmentId: string | undefined) {
  const navigate = useNavigate();

  const [appointment, setAppointment] = useState<Appointment | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isCompleted, setIsCompleted] = useState(false);

  const [sectionsConfig, setSectionsConfig] = useState<NoteSectionConfig[]>([]);
  const [dynamicNotes, setDynamicNotes] = useState<Record<string, string>>({});

  const [recipeSections, setRecipeSections] = useState<Record<string, boolean>>({});
  const [recipeGenerated, setRecipeGenerated] = useState(false);
  const [generatingRecipe, setGeneratingRecipe] = useState(false);
  const [recipeDocDefinition, setRecipeDocDefinition] = useState<any>(null);

  // Días para la cita de control sugerida al finalizar (vacío = sin seguimiento).
  const [followUpDays, setFollowUpDays] = useState('');

  const [patientForm, setPatientFormData] = useState<PatientFormState>(EMPTY_PATIENT_FORM);

  // Diagnóstico estructurado (CIE-10, catálogo oficial DGIS): independiente
  // del texto libre que el doctor escribe en "Notas de la Consulta" — ver
  // el comentario en models.Appointment.DiagnosisCode. diagnosisCodeLabel
  // es solo para mostrar el nombre oficial junto al código en la UI.
  const [diagnosisCode, setDiagnosisCode] = useState('');
  const [diagnosisCodeLabel, setDiagnosisCodeLabel] = useState('');

  const [uploadedFiles, setUploadedFiles] = useState<AppointmentFile[]>([]);
  const [isSyncingFiles, setIsSyncingFiles] = useState(false);
  const [showAutosaveToast, setShowAutosaveToast] = useState(false);

  const lastSavedDataRef = useRef<string>('');
  const isRestoredRef = useRef(false);
  const uploadedFilesRef = useRef(uploadedFiles);
  const isHistoryInterceptedRef = useRef(false);

  // --- Config de apartados de notas (definida por el doctor en Ajustes) ---
  useEffect(() => {
    const savedConfig = localStorage.getItem('doctor_notes_template');
    setSectionsConfig(savedConfig ? JSON.parse(savedConfig) : DEFAULT_SECTIONS);
  }, []);

  useEffect(() => {
    uploadedFilesRef.current = uploadedFiles;
  }, [uploadedFiles]);

  // --- Carga de la cita desde el backend ---
  const loadConsultation = useCallback(async () => {
    if (!appointmentId) return;
    try {
      setLoading(true);
      const response = await api.get(`/appointments/${appointmentId}`);
      const data = response.data;
      setAppointment(data);

      // Regla de negocio: si la cita ya está completada, se carga en modo
      // solo lectura (mismos datos, sin autoguardado ni restauración de
      // borrador) en vez de bloquear la pantalla por completo.
      const isNowCompleted = data.status === 'COMPLETED';
      if (isNowCompleted) {
        localStorage.removeItem(`consultation_draft_${appointmentId}`);
        setIsCompleted(true);
        isRestoredRef.current = true; // Evita disparar la restauración de borrador
      }

      const p = data.patient;
      const h = p?.medicalHistory;

      const initialPatientData: PatientFormState = {
        firstName: p?.firstName || '',
        lastName: p?.lastName || '',
        birthDate: p?.birthDate?.split('T')[0] || '',
        gender: p?.gender || '',
        phone: p?.phone || '',
        email: p?.email || '',
        address: p?.address || '',
        medicalHistory: {
          allergies: h?.allergies || '',
          pathological_history: h?.pathological_history || '',
          non_pathological_history: h?.non_pathological_history || '',
          surgical_history: h?.surgical_history || '',
          current_medication: h?.current_medication || '',
          hereditaryHistory: h?.hereditaryHistory || '',
          gynecoObstetric: h?.gynecoObstetric || '',
          habitsLifestyle: h?.habitsLifestyle || '',
          additional_notes: '',
        },
      };

      setPatientFormData(initialPatientData);

      let initialNotes: Record<string, string> = {};
      if (data.dynamic_notes) {
        initialNotes = typeof data.dynamic_notes === 'string' ? JSON.parse(data.dynamic_notes) : data.dynamic_notes;
      } else if (data.notes) {
        // Compatibilidad con consultas viejas que usaban texto plano
        initialNotes['subjetivo'] = data.notes;
      }
      setDynamicNotes(initialNotes);

      setDiagnosisCode(data.diagnosisCode || '');
      if (data.diagnosisCode) {
        // Resuelve el nombre oficial del código ya guardado, solo para
        // mostrarlo — mejor esfuerzo, si falla se ve el código a secas.
        api.get(`/utils/cie10?q=${encodeURIComponent(data.diagnosisCode)}`)
          .then((res) => {
            const match = (res.data || []).find((c: { code: string }) => c.code === data.diagnosisCode);
            if (match) setDiagnosisCodeLabel(match.name);
          })
          .catch(() => {});
      }

      if (data.documents) {
        const mappedInitialFiles: AppointmentFile[] = data.documents.map((doc: any) => ({
          id: doc.id,
          name: doc.filename || '',
          type: doc.fileType || '',
          size: 0,
          url: toAbsoluteFileUrl(doc.file_path),
          isServerFile: true,
        }));
        setUploadedFiles(mappedInitialFiles);
      }

      lastSavedDataRef.current = JSON.stringify({ patientForm: initialPatientData, dynamicNotes: initialNotes });
    } catch (err: unknown) {
      console.error(err);
      setError(getErrorMessage(err, 'Error al obtener detalles de la consulta'));
    } finally {
      setLoading(false);
    }
  }, [appointmentId]);

  useEffect(() => {
    loadConsultation();
  }, [loadConsultation]);

  // --- Sondeo de archivos cada 20s (por si el paciente sube algo por QR) ---
  const refreshFiles = useCallback(async () => {
    if (!appointmentId) return;
    setIsSyncingFiles(true);
    try {
      const response = await api.get(`/appointments/${appointmentId}`);
      if (response.data.documents) {
        const mappedServerFiles: AppointmentFile[] = response.data.documents.map((doc: any) => ({
          id: doc.id,
          name: doc.filename,
          type: doc.fileType,
          size: 0,
          url: toAbsoluteFileUrl(doc.file_path),
          isServerFile: true,
        }));

        setUploadedFiles((prevFiles) => {
          const localOnlyFiles = prevFiles.filter((f) => !f.isServerFile);
          const localNamesMap = new Map(prevFiles.map((f) => [f.id || f.url, f.name]));
          const combinedServerFiles = mappedServerFiles.map((sf) => {
            const key = sf.id || sf.url;
            return { ...sf, name: localNamesMap.has(key) ? localNamesMap.get(key)! : sf.name };
          });
          return [...combinedServerFiles, ...localOnlyFiles];
        });
      }
    } catch (err) {
      console.error('Error al refrescar archivos:', err);
    } finally {
      setIsSyncingFiles(false);
    }
  }, [appointmentId]);

  useEffect(() => {
    const interval = setInterval(refreshFiles, 20000);
    return () => clearInterval(interval);
  }, [refreshFiles]);

  // --- Restaurar borrador local si difiere de lo que llegó del backend ---
  useEffect(() => {
    if (loading || isRestoredRef.current || !appointmentId) return;

    const savedDraft = localStorage.getItem(`consultation_draft_${appointmentId}`);
    if (savedDraft) {
      try {
        const draft = JSON.parse(savedDraft);
        if (draft?.timestamp) {
          const tieneCambiosProcesables =
            JSON.stringify(draft.patientForm) !== JSON.stringify(patientForm);

          if (tieneCambiosProcesables) {
            const message = `Se encontró un borrador guardado automáticamente el ${new Date(draft.timestamp).toLocaleString()}.\n\n¿Desea recuperar los cambios modificados pendientes?`;
            if (window.confirm(message)) {
              if (draft.patientForm) setPatientFormData(draft.patientForm);
              if (draft.dynamicNotes) setDynamicNotes(draft.dynamicNotes);
              if (draft.uploadedFiles) setUploadedFiles(draft.uploadedFiles);
              lastSavedDataRef.current = JSON.stringify({ patientForm: draft.patientForm, dynamicNotes: draft.dynamicNotes });
            } else {
              localStorage.removeItem(`consultation_draft_${appointmentId}`);
            }
          }
        }
      } catch (e) {
        console.error('Error al parsear el borrador:', e);
      }
    }
    isRestoredRef.current = true;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loading, appointmentId]);

  // --- Autoguardado cada 15s (solo si hay cambios desde el último guardado) ---
  useEffect(() => {
    if (loading || !appointmentId || !isRestoredRef.current || isCompleted) return;

    const saveDraft = () => {
      const currentDataStr = JSON.stringify({ patientForm, dynamicNotes });
      if (currentDataStr === lastSavedDataRef.current) return;

      const draftData = {
        patientForm,
        dynamicNotes,
        uploadedFiles: uploadedFilesRef.current,
        timestamp: new Date().toISOString(),
      };
      localStorage.setItem(`consultation_draft_${appointmentId}`, JSON.stringify(draftData));
      lastSavedDataRef.current = currentDataStr;

      setShowAutosaveToast(true);
      setTimeout(() => setShowAutosaveToast(false), 3000);
    };

    const interval = setInterval(saveDraft, 15000);
    return () => clearInterval(interval);
  }, [loading, appointmentId, patientForm, dynamicNotes, isCompleted]);

  // --- Bloqueo de salida (recarga y navegación) si hay cambios sin guardar ---
  // No aplica en modo solo lectura (cita completada): no hay nada que perder,
  // así que ni siquiera se interceptan beforeunload/popstate.
  useEffect(() => {
    if (isCompleted) return;

    let isHandlingPopState = false;

    const isDirty = () => JSON.stringify({ patientForm, dynamicNotes }) !== lastSavedDataRef.current;

    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (!isDirty()) return;
      e.preventDefault();
      e.returnValue = '¿Estás seguro de que quieres salir? Los cambios de la consulta se perderán.';
      return e.returnValue;
    };

    const originalPushState = window.history.pushState;
    const originalReplaceState = window.history.replaceState;

    const handleInternalNavigation = () => {
      if (!isDirty()) return true;
      return window.confirm(
        '¡Atención! Tienes una consulta médica activa con cambios sin guardar. ¿Estás seguro de que quieres salir y abandonar los datos?'
      );
    };

    if (!isHistoryInterceptedRef.current) {
      window.history.pushState = function (...args: Parameters<History['pushState']>) {
        if (handleInternalNavigation()) {
          return originalPushState.apply(this, args);
        }
        originalPushState.apply(window.history, [null, '', window.location.href]);
      };
      isHistoryInterceptedRef.current = true;
    }

    const handlePopState = () => {
      if (isHandlingPopState) return;
      isHandlingPopState = true;

      if (handleInternalNavigation()) {
        // El navegador ya movió el historial hacia atrás (eso fue lo que
        // disparó este evento popstate) — no hay que navegar de nuevo, o
        // se retrocede dos entradas en vez de una.
        setTimeout(() => {
          isHandlingPopState = false;
        }, 100);
      } else {
        originalPushState.apply(window.history, [null, '', window.location.href]);
        setTimeout(() => {
          isHandlingPopState = false;
        }, 100);
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    window.addEventListener('popstate', handlePopState);

    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
      window.removeEventListener('popstate', handlePopState);
      window.history.pushState = originalPushState;
      window.history.replaceState = originalReplaceState;
      isHistoryInterceptedRef.current = false;
    };
  }, [navigate, patientForm, dynamicNotes, isCompleted]);

  // --- Archivos ---
  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (files) {
      Array.from(files).forEach((file) => {
        const newFile: AppointmentFile = {
          name: file.name,
          type: file.type,
          size: file.size,
          url: URL.createObjectURL(file),
          originalFile: file,
          isServerFile: false,
        };
        setUploadedFiles((prev) => [...prev, newFile]);
      });
    }
  };

  const removeFile = async (file: AppointmentFile) => {
    if (file.isServerFile && file.id) {
      const confirmed = window.confirm(`¿Eliminar permanentemente ${file.name}?`);
      if (confirmed) {
        try {
          await api.delete(`/appointments/${appointmentId}/documents/${file.id}`);
          setUploadedFiles((prev) => prev.filter((f) => f.id !== file.id));
        } catch (err) {
          console.error(err);
          alert('No se pudo eliminar el archivo del servidor.');
        }
      }
    } else {
      setUploadedFiles((prev) => prev.filter((f) => f.name !== file.name));
      if (file.url.startsWith('blob:')) URL.revokeObjectURL(file.url);
    }
  };

  // Apartados que realmente van a aparecer en la receta: marcados como
  // "Incluir en Receta" (true por defecto) y con contenido escrito.
  const sectionsIncludedInRecipe = sectionsConfig.filter(
    (sec) => recipeSections[sec.label] !== false && (dynamicNotes[sec.label] ?? '').trim() !== ''
  );

  // --- Receta: generar, guardar e imprimir en un solo paso ---
  const handleGenerateAndPrintRecipe = async () => {
    if (!appointmentId || appointmentId === 'undefined') {
      alert('Error: No se encontró un ID de cita válido para generar la receta.');
      return;
    }
    if (sectionsIncludedInRecipe.length === 0) {
      alert('Selecciona al menos un apartado con contenido para incluir en la receta antes de generarla (revisa las casillas "Incluir en Receta").');
      return;
    }
    setGeneratingRecipe(true);
    try {
      const doctorRes = await api.get('/doctor/me');
      const doctorInfo = doctorRes.data;
      const patientInfo = appointment?.patient;

      const docDefinition = await generateAndSaveRecipePDF(doctorInfo, patientInfo, appointmentId, {
        diagnosis: appointment?.diagnosis,
        dynamicNotes,
        recipeSections,
      });

      setRecipeDocDefinition(docDefinition);
      setRecipeGenerated(true);
      pdfMake.createPdf(docDefinition).print();
    } catch (err) {
      console.error('Error al generar receta:', err);
      alert('Error al compilar la receta médica.');
    } finally {
      setGeneratingRecipe(false);
    }
  };

  // --- Finalizar consulta ---
  const handleFinalize = async () => {
    for (const sec of sectionsConfig) {
      if (sec.required && !dynamicNotes[sec.label]?.trim()) {
        alert(`El campo "${sec.label}" es obligatorio para finalizar la consulta.`);
        return;
      }
    }

    if (!diagnosisCode.trim()) {
      alert('Debes asociar un código CIE-10 al diagnóstico (sección "Diagnóstico (CIE-10)") antes de finalizar la consulta.');
      return;
    }

    if (patientForm.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(patientForm.email)) {
      alert('El correo electrónico del paciente no tiene un formato válido.');
      return;
    }

    if (!window.confirm('¿Está seguro de finalizar la consulta? Se generará la receta y se cerrará el expediente de esta cita.')) {
      return;
    }

    setLoading(true);
    try {
      const patientUpdatePayload = {
        firstName: patientForm.firstName,
        lastName: patientForm.lastName,
        birthDate: patientForm.birthDate,
        gender: patientForm.gender,
        phone: patientForm.phone,
        email: patientForm.email,
        address: patientForm.address,
        medicalHistory: {
          allergies: patientForm.medicalHistory.allergies,
          pathological_history: patientForm.medicalHistory.pathological_history,
          non_pathological_history: patientForm.medicalHistory.non_pathological_history,
          surgical_history: patientForm.medicalHistory.surgical_history,
          current_medication: patientForm.medicalHistory.current_medication,
          hereditaryHistory: patientForm.medicalHistory.hereditaryHistory,
          gynecoObstetric: patientForm.medicalHistory.gynecoObstetric,
          habitsLifestyle: patientForm.medicalHistory.habitsLifestyle,
        },
      };

      const patientId = appointment?.patient?.id;
      await api.put(`/patients/${patientId}`, patientUpdatePayload);

      const serverFilesToUpdate = uploadedFiles.filter((f) => f.isServerFile && f.id);
      for (const file of serverFilesToUpdate) {
        await api.put(`/appointments/${appointmentId}/documents/${file.id}`, { filename: file.name });
      }

      // Archivos adjuntados localmente durante la consulta (pendientes de subir).
      const localFiles = uploadedFiles.filter((f) => !f.isServerFile && f.originalFile);
      if (localFiles.length > 0) {
        const formData = new FormData();
        localFiles.forEach((f) => formData.append('files', f.originalFile!, f.name));
        formData.append('isPrescription', 'false');
        await api.post(`/appointments/${appointmentId}/upload-document`, formData, {
          headers: { 'Content-Type': 'multipart/form-data' },
        });
      }

      const followUpDaysNumber = parseInt(followUpDays, 10);
      const followUpDate =
        Number.isFinite(followUpDaysNumber) && followUpDaysNumber > 0
          ? new Date(Date.now() + followUpDaysNumber * 24 * 60 * 60 * 1000).toISOString()
          : undefined;

      await api.put(`/appointments/${appointmentId}`, {
        ...appointment,
        status: 'COMPLETED',
        dynamic_notes: dynamicNotes,
        diagnosisCode,
        ...(diagnosisCodeLabel ? { diagnosis: diagnosisCodeLabel } : {}),
        ...(followUpDate ? { followUpDate } : {}),
      });

      const docRes = await api.get('/doctor/me');
      await generateAndSaveRecipePDF(docRes.data, appointment?.patient, appointmentId!, {
        diagnosis: appointment?.diagnosis,
        dynamicNotes,
        recipeSections,
      });

      localStorage.removeItem(`consultation_draft_${appointmentId}`);
      // Evita que el bloqueo de navegación (más abajo) pregunte "¿seguro que
      // quieres salir?" sobre la propia navegación de salida tras finalizar.
      lastSavedDataRef.current = JSON.stringify({ patientForm, dynamicNotes });
      navigate('/inicio');
    } catch (err) {
      console.error('Error al cerrar consulta:', err);
      alert(getErrorMessage(err, 'Ocurrió un error al intentar finalizar la consulta. Los cambios no se guardaron por completo.'));
    } finally {
      setLoading(false);
    }
  };

  return {
    appointment,
    loading,
    error,
    isCompleted,
    sectionsConfig,
    dynamicNotes,
    setDynamicNotes,
    recipeSections,
    setRecipeSections,
    recipeGenerated,
    generatingRecipe,
    hasSectionsForRecipe: sectionsIncludedInRecipe.length > 0,
    followUpDays,
    setFollowUpDays,
    patientForm,
    setPatientFormData,
    diagnosisCode,
    setDiagnosisCode,
    diagnosisCodeLabel,
    setDiagnosisCodeLabel,
    uploadedFiles,
    isSyncingFiles,
    showAutosaveToast,
    handleFileUpload,
    removeFile,
    handleGenerateAndPrintRecipe,
    handleFinalize,
  };
}
