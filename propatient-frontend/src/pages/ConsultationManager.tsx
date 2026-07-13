import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
// import pdfMake from 'pdfmake';
// import pdfFonts from 'pdfmake/build/vfs_fonts';
import type { Patient, Appointment, MedicalHistory, ConsultationNotes } from '../types';
import './ConsultationManager.scss';
import api from '../api/axios';
import pdfMake from 'pdfmake/build/pdfmake';
import * as pdfFonts from 'pdfmake/build/vfs_fonts';

// Sincronización robusta para el bundle de Vite
if (pdfFonts && pdfFonts.pdfMake) {
  (pdfMake as any).vfs = pdfFonts.pdfMake.vfs;
} else if ((pdfFonts as any).vfs) {
  (pdfMake as any).vfs = (pdfFonts as any).vfs;
}

// Forzar inyección en el objeto global de la ventana por si la librería lo busca ahí internamente
if (window) {
  (window as any).pdfMake = pdfMake;
  (window as any).pdfMake.vfs = (pdfMake as any).vfs;
}

interface AppointmentFile {
  id?: number;
  name: string;
  type: string;
  size: number;
  url: string;
  originalFile?: File;
  isServerFile?: boolean;
}

interface NoteSectionConfig {
  id: string;
  label: string;
  placeholder: string;
  required: boolean;
}

const baseurl = "http://localhost:8095"

type FormSection = 'generalData' | 'medicalHistory';

export const ConsultationManager: React.FC = () => {
  const { appointmentId } = useParams<{ appointmentId: string }>();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  
  const [appointment, setAppointment] = useState<Appointment | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [selectedSidebarFile, setSelectedSidebarFile] = useState<AppointmentFile | null>(null);
  const [sidebarWidth, setSidebarWidth] = useState<number>(380); // Ancho inicial en píxeles
  const isResizingRef = useRef<boolean>(false);

  // --- REFERENCIA PARA CONTROL DE CAMBIOS (isDirty) ---
  const lastSavedDataRef = useRef<string>("");
  const currentChangesRef = useRef({ reason: '', notes: '' });

  // --- ESTADOS DEL FORMULARIO ---
  const [activeTab, setActiveTab] = useState<FormSection>('generalData');

  const [sectionsConfig, setSectionsConfig] = useState<NoteSectionConfig[]>([]);
  const [dynamicNotes, setDynamicNotes] = useState<Record<string, string>>({});

  const [recipeSections, setRecipeSections] = useState<Record<string, boolean>>({});
  
  const [recipeGenerated, setRecipeGenerated] = useState<boolean>(false);
  const [generatingRecipe, setGeneratingRecipe] = useState<boolean>(false);
  const [recipeDocDefinition, setRecipeDocDefinition] = useState<any>(null);

  // Datos del Paciente (Editables)
  const [patientForm, setPatientFormData] = useState({
    firstName: '',
    lastName: '',
    birthDate: '',
    gender: '',
    phone: '',
    email: '',
    address: '',
    medicalHistory: {
// Campos base existentes:
    allergies: '',
    pathological_history: '',
    non_pathological_history: '',
    surgical_history: '',
    current_medication: '',
    // Heredofamiliares
    hereditaryHistory: '',
    // Ginecoobstétricos (pueden dejarse en un string plano compacto para simplificar)
    gynecoObstetric: '', 
    // Estilo de vida / Hábitos
    habitsLifestyle: '',
    // Notas extra
    additional_notes: ''
    }
  });

  // Notas de la Consulta Actual
  const [consultationNotes, setConsultationNotes] = useState<ConsultationNotes>({
    subjective: '',
    objective: '',
    diagnosis: '',
    treatmentPlan: ''
  });

  // --- ESTADOS DE ARCHIVOS Y QR ---
  const [uploadedFiles, setUploadedFiles] = useState<AppointmentFile[]>([]);
  const [isSyncingFiles, setIsSyncingFiles] = useState(false);
  const [showQR, setShowQR] = useState(false);
  const [showAutosaveToast, setShowAutosaveToast] = useState(false);
  const [qrImageUrl, setQrImageUrl] = useState('');

  // --- REFERENCIAS DE CONTROL CRUCIALES ---
  const [isCompleted, setIsCompleted] = useState(false);
  const isRestoredRef = useRef(false); // Evita que se pregunte más de una vez
  const uploadedFilesRef = useRef(uploadedFiles);
  const isHistoryInterceptedRef = useRef(false);

  useEffect(() => {
  const savedConfig = localStorage.getItem('doctor_notes_template');
  if (savedConfig) {
    setSectionsConfig(JSON.parse(savedConfig));
  } else {
    // Si no ha ido a ajustes, cargamos un esquema base predeterminado
    const defaultConfig = [
      { id: 'subjetivo', label: 'Padecimiento Actual (Subjetivo)', placeholder: 'Lo que el paciente refiere...', required: false },
      { id: 'diagnostico', label: 'Diagnóstico', placeholder: 'Impresión diagnóstica...', required: true },
      { id: 'tratamiento', label: 'Plan y Tratamiento', placeholder: 'Receta e indicaciones...', required: true }
    ];
    setSectionsConfig(defaultConfig);
  }
}, []);
  
  useEffect(() => {
    uploadedFilesRef.current = uploadedFiles;
  }, [uploadedFiles]);

  // --- 1. FUNCIÓN DE CARGA DE RED (GO BACKEND) ---
  const loadConsultation = useCallback(async () => {
    try {
      setLoading(true);
      const response = await api.get(`/appointments/${appointmentId}`);
      const data = response.data;
      setAppointment(data);

      // ◄ REGLA DE NEGOCIO: Si la cita ya está completada, bloqueamos y limpiamos residuos locales
      if (data.status === 'COMPLETED' || data.Status === 'COMPLETED') {
        localStorage.removeItem(`consultation_draft_${appointmentId}`);
        setIsCompleted(true);
        isRestoredRef.current = true; // Evita activar el respaldo local
        setLoading(false);
        return; // Salimos de la función inmediatamente
      }

      const p = data.patient || data.Patient;
      const h = p?.MedicalHistory || p?.medicalHistory;

      const initialPatientData: Patient = {
        id: p?.id || 0,
        firstName: p?.firstName || p?.FirstName || '',
        lastName: p?.lastName || p?.LastName || '',
        birthDate: (p?.birthDate || p?.BirthDate)?.split('T')[0] || '',
        gender: p?.gender || p?.Gender || '',
        phone: p?.phone || p?.Phone || '',
        email: p?.email || p?.Email || '',
        address: p?.address || p?.Address || '',
        medicalHistory: {
          hereditaryHistory: h?.hereditaryHistory || h?.hereditary_history || '',
          allergies: h?.allergies || h?.Allergies || '',
          pathological_history: h?.pathological_history || h?.pathologicalHistory || '',
          non_pathological_history: h?.non_pathological_history || h?.nonPathologicalHistory || '',
          surgical_history: h?.surgical_history || h?.surgicalHistory || '',
          current_medication: h?.current_medication || h?.currentMedication || '',
          gynecoObstetric: h?.gynecoObstetric || h?.gyneco_obstetric || '',
          habitsLifestyle: h?.habitsLifestyle || h?.habits_lifestyle || ''
        }
      };

      setPatientFormData(initialPatientData);
      
      //const initialNotes = {
      //  reason: data.reason || '',
      //  subjective: data.notes?.includes('SUBJETIVO:') ? data.notes.split('\n')[0].replace('SUBJETIVO: ', '') : '',
      //  objective: data.notes?.includes('OBJECTIVO:') ? data.notes.split('\n')[1].replace('OBJECTIVO: ', '') : '',
      //  diagnosis: data.diagnosis || '',
      //  treatmentPlan: data.treatmentPlan || ''
      //};
      //setConsultationNotes(initialNotes);
      
      let initialNotes: Record<string, string> = {};
      if (data.dynamic_notes) {
        // Si tu API de Go ya te manda el objeto JSON estructurado directamente
        initialNotes = typeof data.dynamic_notes === 'string' ? JSON.parse(data.dynamic_notes) : data.dynamic_notes;
      } else if (data.notes) {
        // Fallback para consultas viejas que usaban el texto plano estructurado viejo
        initialNotes['subjetivo'] = data.notes;
      }

      setDynamicNotes(initialNotes);


      if (data.documents) {
        const mappedInitialFiles = data.documents.map((doc: any) => ({
          id: doc.id,
          name: doc.filename || doc.FileName || '',
          type: doc.fileType || '',
          size: 0,
          url: `${baseurl}${doc.file_path || ''}`,
          isServerFile: true
        }));
        setUploadedFiles(mappedInitialFiles);
      }

      lastSavedDataRef.current = JSON.stringify({ patientForm: initialPatientData, consultationNotes: initialNotes });

    } catch (err: any) {
      console.error(err);
      setError(err.response?.data?.error || 'Error al obtener detalles de la consulta');
    } finally {
      setLoading(false);
    }
  }, [appointmentId]);

  // --- 2. EFFECT DE MONTAJE INICIAL (UNA SOLA EJECUCIÓN) ---
  useEffect(() => {
    loadConsultation();
  }, [appointmentId]); // Cambiado dependencias para evitar disparos cíclicos

  // --- 3. EVALUACIÓN Y RESTAURACIÓN DEL RESPALDO (SE DISPARA AL APAGAR EL LOADING) ---
useEffect(() => {
    if (loading || isRestoredRef.current || !appointmentId) return;

    const savedDraft = localStorage.getItem(`consultation_draft_${appointmentId}`);
    if (savedDraft) {
      try {
        const draft = JSON.parse(savedDraft);
        if (draft && draft.timestamp) {
          
          // En lugar de comparar strings complejos que fallan por formato, 
          // validamos si de verdad hay diferencias clave con lo que llegó de la BD de Go
          const tieneCambiosProcesables = 
            JSON.stringify(draft.patientForm) !== JSON.stringify(patientFormData) ||
            JSON.stringify(draft.consultationNotes) !== JSON.stringify(consultationNotes);

          if (tieneCambiosProcesables) {
            const message = `Se encontró un borrador guardado automáticamente el ${new Date(draft.timestamp).toLocaleString()}.\n\n¿Desea recuperar los cambios modificados pendientes?`;
            
            if (window.confirm(message)) {
              if (draft.patientForm) setPatientFormData(draft.patientForm);
              if (draft.consultationNotes) setConsultationNotes(draft.consultationNotes);
              if (draft.uploadedFiles) setUploadedFiles(draft.uploadedFiles);
              
              // Sincronizamos la referencia con los datos del borrador que acabamos de inyectar
              lastSavedDataRef.current = JSON.stringify({ 
                patientForm: draft.patientForm, 
                consultationNotes: draft.consultationNotes 
              });
            } else {
              // Si el usuario decide ignorarlo, limpiamos para no molestar más
              localStorage.removeItem(`consultation_draft_${appointmentId}`);
            }
          }
        }
      } catch (e) {
        console.error("Error al parsear el borrador:", e);
      }
    }
    isRestoredRef.current = true;
  }, [loading, appointmentId]); // Quitamos las dependencias mutables de los formularios aquí

  // --- 4. CICLO DE AUTOGUARDADO ACTIVO (CADA 15 SEGUNDOS) ---
 useEffect(() => {
    if (loading || !appointmentId || !isRestoredRef.current) return;

    const saveDraft = () => {
      const currentDataStr = JSON.stringify({ patientForm, consultationNotes });
      
      // Si visualmente los inputs siguen idénticos al último estado guardado en memoria, salimos
      if (currentDataStr === lastSavedDataRef.current) return;

      const draftData = {
        patientForm,
        consultationNotes,
        uploadedFiles: uploadedFilesRef.current,
        timestamp: new Date().toISOString(),
      };
      
      localStorage.setItem(`consultation_draft_${appointmentId}`, JSON.stringify(draftData));
      lastSavedDataRef.current = currentDataStr; // Actualizamos la marca de tiempo del cambio

      setShowAutosaveToast(true);
      setTimeout(() => setShowAutosaveToast(false), 3000);
    };

    const interval = setInterval(saveDraft, 15000);
    return () => clearInterval(interval);
  }, [loading, appointmentId, patientForm, consultationNotes]);

  // --- BLOQUEO DE RECARGA / CIERRE DE PESTAÑA (Navegador) ---
  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      // Solo bloqueamos si hay algún dato cargado o modificado
      e.preventDefault();
      e.returnValue = '¿Estás seguro de que quieres salir? Los cambios no guardados se perderán.';
      return e.returnValue;
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
    };
  }, []);
    
  // --- BLOQUEO DE NAVEGACIÓN TOTAL (INTERNA Y EXTERNA) ---
  // --- BLOQUEO DE NAVEGACIÓN TOTAL (OPTIMIZADO SIN DEPENDENCIAS DE ESTADO) ---
  useEffect(() => {
    let isHandlingPopState = false;

    // 1. Bloqueo para recarga de página (F5) o cerrar pestaña
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      // Si no quieres complicarte con JSONs, un chequeo rápido de texto
      const isDirty = consultationNotes.notes !== "" || (appointment?.reason && appointment.reason !== "");
      if (!isDirty) return;
      
      e.preventDefault();
      e.returnValue = '¿Estás seguro de que quieres salir? Los cambios de la consulta se perderán.';
      return e.returnValue;
    };

    // Guardamos las referencias a los métodos originales de forma segura
    const originalPushState = window.history.pushState;
    const originalReplaceState = window.history.replaceState;

    const handleInternalNavigation = () => {
      const isDirty = consultationNotes.notes !== "" || (appointment?.reason && appointment.reason !== "");
      if (!isDirty) return true; // Permite navegar directo si no hay texto nuevo
      
      return window.confirm(
        "¡Atención! Tienes una consulta médica activa con cambios sin guardar. ¿Estás seguro de que quieres salir y abandonar los datos?"
      );
    };

    // INTERCEPCIÓN SEGURA: Solo modificamos el prototipo si no se ha hecho antes en este ciclo
    if (!isHistoryInterceptedRef.current) {
      window.history.pushState = function (...args) {
        if (handleInternalNavigation()) {
          return originalPushState.apply(this, args);
        }
        originalPushState.apply(window.history, [null, '', window.location.href]);
      };
      isHistoryInterceptedRef.current = true;
    }

    // 2. Interceptamos el botón "Atrás" del propio navegador
    const handlePopState = () => {
      if (isHandlingPopState) return;
      isHandlingPopState = true;

      if (handleInternalNavigation()) {
        window.removeEventListener('popstate', handlePopState);
        navigate(-1);
      } else {
        originalPushState.apply(window.history, [null, '', window.location.href]);
        setTimeout(() => {
          isHandlingPopState = false;
        }, 100);
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    window.addEventListener('popstate', handlePopState);

    // Limpieza absoluta al desmontar la pantalla de consulta por completo
    return () => {
      window.removeEventListener('beforeunload', handleBeforeUnload);
      window.removeEventListener('popstate', handlePopState);
      window.history.pushState = originalPushState;
      window.history.replaceState = originalReplaceState;
      isHistoryInterceptedRef.current = false;
    };
  }, [navigate, appointment?.reason, consultationNotes.notes]); // ◄ SOLUCIÓN: Solo depende de navigate, nunca volverá a fallar al mutar otros estados

// --- MANEJADOR PARA REDIMENSIONAR EL PANEL LATERAL ---
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizingRef.current) return;
      // Calculamos el nuevo ancho restando la posición del mouse del borde derecho
      const newWidth = window.innerWidth - e.clientX;
      // Ponemos límites mínimos y máximos para que no rompa el diseño
      if (newWidth > 250 && newWidth < 800) {
        setSidebarWidth(newWidth);
      }
    };

    const handleMouseUp = () => {
      isResizingRef.current = false;
      document.body.style.cursor = 'default';
      document.body.style.userSelect = 'auto';
    };

    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);

    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('mouseup', handleMouseUp);
    };
  }, []);

  const startResizing = (e: React.MouseEvent) => {
    e.preventDefault();
    isResizingRef.current = true;
    document.body.style.cursor = 'ew-resize';
    document.body.style.userSelect = 'none'; // Evita seleccionar texto al arrastrar
  };


  const mapServerFiles = (docs: any[]) => {
    const files: AppointmentFile[] = docs.map(doc => ({
      id: doc.id,
      name: doc.filename,
      type: doc.fileType,
      size: doc.data ? Math.round(doc.data.length * 0.75) : 0,
      url: `${baseurl}${doc.file_path}`,
      isServerFile: true
    }));
    setUploadedFiles(files);
  };

  const refreshFiles = useCallback(async () => {
    if (!appointmentId) return;
    setIsSyncingFiles(true);
    try {
      const response = await api.get(`/appointments/${appointmentId}`);
      if (response.data.documents) {
        const serverDocs = response.data.documents;
        
        const mappedServerFiles: AppointmentFile[] = serverDocs.map((doc: any) => ({
          id: doc.id,
          name: doc.filename || doc.FileName || doc.fileName,
          type: doc.fileType,
          size: doc.data ? Math.round(doc.data.length * 0.75) : 0,
          url: `${baseurl}${doc.file_path}`,
          isServerFile: true // ◄ CAMBIO AQUÍ: Agrega esta bandera obligatoria
        }));

        setUploadedFiles(prevFiles => {
          const localOnlyFiles = prevFiles.filter(f => !f.isServerFile);
          const localNamesMap = new Map(prevFiles.map(f => [f.id || f.url, f.name]));

          const combinedServerFiles = mappedServerFiles.map(sf => {
            const key = sf.id || sf.url;
            return {
              ...sf,
              name: localNamesMap.has(key) ? localNamesMap.get(key)! : sf.name
            };
          });

          return [...combinedServerFiles, ...localOnlyFiles];
        });
      }
    } catch (error) {
      console.error("Error al refrescar archivos:", error);
    } finally {
      setIsSyncingFiles(false);
    }
  }, [appointmentId]);

  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (files) {
      Array.from(files).forEach(file => {
        const newFile: AppointmentFile = {
          name: file.name,
          type: file.type,
          size: file.size,
          url: URL.createObjectURL(file),
          originalFile: file,
          isServerFile: false
        };
        setUploadedFiles(prev => [...prev, newFile]);
      });
    }
  };

const getBase64FromUrl = (url: string): Promise<string> => {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.crossOrigin = 'anonymous'; // Enfoque nativo para evitar bloqueos XHR
    img.src = url;
    
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = img.width;
      canvas.height = img.height;
      const ctx = canvas.getContext('2d');
      if (ctx) {
        ctx.drawImage(img, 0, 0);
        resolve(canvas.toDataURL('image/png'));
      } else {
        reject(new Error('No se pudo procesar el canvas'));
      }
    };
    img.onerror = (error) => reject(error);
  });
};

  const removeFile = async (file: AppointmentFile) => {
    if (file.isServerFile && file.id) {
      const confirmed = window.confirm(`¿Eliminar permanentemente ${file.name}?`);
      if (confirmed) {
        try {
          await api.delete(`/appointments/${appointmentId}/documents/${file.id}`);
          setUploadedFiles(prev => prev.filter(f => f.id !== file.id));
        } catch (err) {
          alert("No se pudo eliminar el archivo del servidor.");
        }
      }
    } else {
      setUploadedFiles(prev => prev.filter(f => f.name !== file.name));
      if (file.url.startsWith('blob:')) URL.revokeObjectURL(file.url);
    }
  };

  const handleRenameFile = (index: number, newName: string) => {
    setUploadedFiles(prevFiles => {
      const updated = [...prevFiles];
      if (updated[index]) {
        updated[index] = {
          ...updated[index],
          name: newName,
          isEdited: true // Le ponemos una marca temporal para saber que lo estás modificando
        };
      }
      return updated;
    });
  };


  const toggleQR = () => {
    if (!showQR) {
      const uploadUrl = `${window.location.origin}/public-upload/${appointmentId}`;
      setQrImageUrl(`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(uploadUrl)}`);
    }
    setShowQR(!showQR);
  };

// Añade esta pequeña referencia justo arriba del useEffect de autoguardado para no crear bucles
  useEffect(() => {
    uploadedFilesRef.current = uploadedFiles;
  }, [uploadedFiles]);

  useEffect(() => {
    currentChangesRef.current = {
      reason: appointment?.reason || '',
      notes: consultationNotes.notes || ''
    };

    if (loading || !appointmentId) return;

    const saveDraft = () => {
      const currentData = {
        patientForm,
        consultationNotes,
        uploadedFiles: uploadedFilesRef.current // Uso seguro de la referencia
      };
      
      const currentDataStr = JSON.stringify({ patientForm, consultationNotes });
      if (currentDataStr === lastSavedDataRef.current) return;

      const draftData = {
        ...currentData,
        timestamp: new Date().toISOString(),
      };
      
      localStorage.setItem(`consultation_draft_${appointmentId}`, JSON.stringify(draftData));
      lastSavedDataRef.current = currentDataStr;

      setShowAutosaveToast(true);
      setTimeout(() => setShowAutosaveToast(false), 3000);
    };

    // Cambiado a 15 segundos para que responda más rápido en tus pruebas
    const interval = setInterval(saveDraft, 15000); 
    return () => clearInterval(interval);
  }, [loading, appointmentId, patientForm, consultationNotes]);

 // 1. Manejador para GENERAR y GUARDAR la receta (Paso 1)
  const handleCreateRecipeClick = async () => {
    setGeneratingRecipe(true);
    try {
      const doctorRes = await api.get('/doctor/me');
      const doctorInfo = doctorRes.data;
      const patientInfo = appointment?.patient;

      // 🌟 SOLUCIÓN: Nos aseguramos de usar 'appointmentId' que viene de useParams
      if (!appointmentId || appointmentId === "undefined") {
        alert("Error: No se encontró un ID de cita válido para generar la receta.");
        return;
      }

      let doctorLogoBase64 = "";
      if (doctorInfo?.logoUrl) {
        try {
          // 🌟 Tu lógica exacta: Validamos si ya viene con http o si le pegamos el baseurl (BACKEND_URL)
          const cleanFullUrl = doctorInfo.logoUrl.startsWith('http') 
            ? doctorInfo.logoUrl 
            : `${baseurl}${doctorInfo.logoUrl}`;
          
          console.log("📥 Convirtiendo logo a Base64 desde:", cleanFullUrl);
          
          // Convertimos la imagen de forma segura a Base64
          doctorLogoBase64 = await getBase64FromUrl(cleanFullUrl);
          
        } catch (err) {
          console.error("No se pudo mapear el logo del doctor, usando respaldo de texto:", err);
          doctorLogoBase64 = ""; // Si falla, mantendrá el texto de respaldo para que no truene la receta
        }
      }

      const currentRecipeSections = recipeSections || {};
      const recipeContent = Object.keys(dynamicNotes)
        .filter(label => currentRecipeSections[label] !== false && dynamicNotes[label]?.trim() !== '')
        .map(label => [
          { text: label.toUpperCase(), style: 'sectionHeader' },
          { text: dynamicNotes[label], style: 'sectionBody' },
          { text: '\n' }
        ]).flat();

      const hasValidBase64 = doctorLogoBase64 && doctorLogoBase64.startsWith('data:image');

      // Creamos el docDefinition definitivo
      const docDefinition: any = {
        pageSize: 'LETTER',
        pageMargins: [40, 40, 40, 80],
        defaultStyle: { font: 'Roboto' },
        content: [
          {
            columns: [
              hasValidBase64 
                ? { image: doctorLogoBase64, width: 90, alignment: 'left' } 
                : { text: 'MÉDICO GENERAL', fontSize: 14, bold: true, color: '#1a365d', margin: [0, 15, 0, 0] },
              [
                { text: `DR. ${doctorInfo?.firstName || ''} ${doctorInfo?.lastName || ''}`.toUpperCase(), style: 'doctorName' },
                { text: `${doctorInfo?.specialty || 'MÉDICO CIRUJANO Y PARTERO'}`, style: 'doctorSpecialty' },
                { text: `CÉDULA PROFESIONAL: ${doctorInfo?.professionalId || 'N/A'}`, style: 'doctorSub' },
                { text: `${doctorInfo?.institution || 'UNIVERSIDAD AUTÓNOMA DE SINALOA'}`, style: 'doctorSub' },
              ]
            ],
            columnGap: 20
          },
          { canvas: [{ type: 'line', x1: 0, y1: 15, x2: 532, y2: 15, lineWidth: 2, lineColor: '#1a365d' }] },
          { text: '\n' },
          {
            style: 'patientTable',
            table: {
              widths: ['*', 120, 80],
              body: [
                [
                  { text: `PACIENTE: ${patientInfo?.firstName || ''} ${patientInfo?.lastName || ''}`.toUpperCase(), style: 'tableCellBold' },
                  { text: `EDAD: ${patientInfo?.age || 'N/A'} AÑOS`, style: 'tableCell' },
                  { text: `FECHA: ${new Date().toLocaleDateString()}`, style: 'tableCell', alignment: 'right' }
                ],
                [
                  { text: `DIAGNÓSTICO: ${appointment?.diagnosis || 'Sintomatología general'}`, style: 'tableCell', colSpan: 3 },
                  {}, {}
                ]
              ]
            },
            layout: {
              hLineWidth: () => 0.5, vLineWidth: () => 0.5,
              hLineColor: () => '#cbd5e0', vLineColor: () => '#cbd5e0',
              paddingTop: () => 6, paddingBottom: () => 6,
              paddingLeft: () => 8, paddingRight: () => 8,
            }
          },
          { text: '\n\n' },
          ...recipeContent,
        ],
        footer: () => {
          return {
            stack: [
              { text: '_______________________________________', alignment: 'center', color: '#cbd5e0' },
              { text: `DR. ${doctorInfo?.firstName || ''} ${doctorInfo?.lastName || ''}`.toUpperCase(), alignment: 'center', fontSize: 10, bold: true, color: '#2d3748', margin: [0, 2, 0, 2] },
              { text: 'FIRMA DEL MÉDICO', alignment: 'center', fontSize: 8, color: '#718096' },
              { text: `Dirección: ${doctorInfo?.address || 'Av. De la Clínica #123'} | Tel: ${doctorInfo?.phone || 'N/A'}`, alignment: 'center', fontSize: 8, color: '#718096', margin: [0, 4, 0, 0] }
            ],
            margin: [40, 0, 40, 0]
          };
        },
        styles: {
          doctorName: { fontSize: 16, bold: true, color: '#1a365d', alignment: 'right' },
          doctorSpecialty: { fontSize: 10, bold: true, color: '#4a5568', alignment: 'right' },
          doctorSub: { fontSize: 9, color: '#718096', alignment: 'right' },
          patientTable: { margin: [0, 5, 0, 15] },
          tableCell: { fontSize: 10, color: '#2d3748' },
          tableCellBold: { fontSize: 10, bold: true, color: '#1a365d' },
          sectionHeader: { fontSize: 11, bold: true, color: '#1a365d', margin: [0, 10, 0, 4], decoration: 'underline' },
          sectionBody: { fontSize: 11, color: '#2d3748', marginLeft: 10 }
        }
      };

      // Guardamos la definición en el estado para que el botón de imprimir la use
      setRecipeDocDefinition(docDefinition);

      // Enviamos el archivo al backend usando el ID correcto
      const pdfInstance = pdfMake.createPdf(docDefinition);
      const blob = await pdfInstance.getBlob();
      const formData = new FormData();
      const fileName = `receta_${appointmentId}_doc_${doctorInfo?.serialCode || '0'}.pdf`;
      formData.append('recipe_pdf', blob, fileName);

      await api.post(`/appointments/${appointmentId}/save-recipe-pdf`, formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
      });

      setRecipeGenerated(true);
      alert("🎉 Receta generada y guardada con éxito.");
    } catch (error) {
      console.error("Error al generar receta:", error);
      alert("Error al compilar la receta médica.");
    } finally {
      setGeneratingRecipe(false);
    }
  };

  // 2. Manejador para IMPRIMIR (Paso 2)
  const handlePrintRecipeClick = () => {
    if (recipeDocDefinition) {
      pdfMake.createPdf(recipeDocDefinition).print();
    } else {
      alert("Por favor, primero genera la receta en el Paso 1.");
    }
  };


  const handleFinalize = async () => {
    
    for (const sec of sectionsConfig) {
      if (sec.required && !dynamicNotes[sec.label]?.trim()) {
        alert(`El campo "${sec.label}" es obligatorio para finalizar la consulta.`);
        return;
      }
    }

    if (patientForm.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(patientForm.email)) {
      alert("El correo electrónico del paciente no tiene un formato válido.");
      return;
    }

    const confirmClose = window.confirm("¿Está seguro de finalizar la consulta? Se generará la receta y se cerrará el expediente de esta cita.");
    if (!confirmClose) return;

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

        }
      };
      
      const patientId = appointment?.patient?.id || appointment?.Patient?.id || (appointment?.patient as any)?.ID;
      await api.put(`/patients/${patientId}`, patientUpdatePayload);

      const serverFilesToUpdate = uploadedFiles.filter(f => f.id !== undefined && f.id !== null);
      
      console.log("Archivos del servidor encontrados para actualizar:", serverFilesToUpdate); // Para tu debug
      for (const file of serverFilesToUpdate) {
        // Asumiendo la ruta estándar REST para actualizar metadatos del documento
        await api.put(`/appointments/${appointmentId}/documents/${file.id}`, {
          filename: file.name
        });
      }

      // 3. Subir archivos nuevos locales renombrados (si existen)
      const localFiles = uploadedFiles.filter(f => !f.isServerFile && f.id);
      if (localFiles.length > 0) {
        const formData = new FormData();
        localFiles.forEach(f => {
          // Pasamos el f.name modificado en el tercer parámetro del append para que Multer/Go lo reciba con el nuevo nombre
          formData.append('files', f.originalFile!, f.name);
        });
        formData.append('isPrescription', 'false');
        await api.post(`/appointments/${appointmentId}/upload-document`, formData, {
          headers: { 'Content-Type': 'multipart/form-data' }
        });
      }

      const appointmentUpdatePayload = {
        ...appointment,
        status: 'COMPLETED',
        dynamic_notes: dynamicNotes,
      };
      await api.put(`/appointments/${appointmentId}`, appointmentUpdatePayload);

      // 2. Traemos la info del Doctor logueado (que incluye cédula, logo, serial, etc.)
      const docRes = await api.get('/doctor/me');
      const doctorInfo = docRes.data;

      // 3. Corremos el proceso dinámico de PDF (Imprime y sube el archivo con nombre único)
      await generateAndSaveRecipePDF(doctorInfo, appointment.patient, appointmentId);

      localStorage.removeItem(`consultation_draft_${appointmentId}`);
      window.onbeforeunload = null;
      navigate('/inicio');
    } catch (err) {
      console.error("Error al cerrar consulta:", err);
      alert("Ocurrió un error al intentar finalizar la consulta. Los cambios no se guardaron por completo.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const init = async () => {
      if (appointmentId) {
        await loadConsultation();
      }
    };
    init();
    const interval = setInterval(refreshFiles, 20000);
    return () => clearInterval(interval);
  }, [appointmentId, loadConsultation]);

  if (loading) return <div className="consultation-manager-container loading-state"><div className="spinner"></div><p>Iniciando consulta médica...</p></div>;
  if (error) {
      return (
        <div className="consultation-error">
          <span className="material-icons-outlined">error_outline</span>
          <p>{error}</p>
          <button className="btn-primary" onClick={() => navigate('/inicio')}>Volver al Inicio</button>
        </div>
      );
    

    if (isCompleted) {
    const patientId = appointment?.patient?.id || appointment?.Patient?.id || (appointment?.patient as any)?.ID;
    return (
      <div className="consultation-completed-lock-overlay">
        <div className="lock-card">
          <span className="material-icons-outlined lock-icon">task_alt</span>
          <h2>Esta cita ya fue completada</h2>
          <p>
            Los datos clínicos de esta consulta han sido guardados de manera definitiva en el expediente electrónico y no pueden ser modificados.
          </p>
          <p className="hint">
            Por favor, vaya al historial del paciente para poder visualizar el resumen completo de su evolución.
          </p>
          <div className="lock-actions">
            <button className="btn-secondary" onClick={() => navigate('/inicio')}>
              Ir al Inicio
            </button>
            {patientId && (
              <button className="btn-primary" onClick={() => navigate(`/patients/${patientId}/history`)}>
                Ver Historial del Paciente
              </button>
            )}
          </div>
        </div>
      </div>
    );
  }
  
    return (
      <div className="consultation-completed-lock-overlay">
        <div className="lock-card">
          <span className="material-icons-outlined lock-icon">task_alt</span>
          <h2>Esta cita ya fue completada</h2>
          <p>
            Los datos clínicos de esta consulta han sido guardados de manera definitiva en el expediente electrónico y no pueden ser modificados.
          </p>
          <p className="hint">
            Por favor, vaya al historial del paciente para poder visualizar el resumen completo de su evolución.
          </p>
          <div className="lock-actions">
            <button className="btn-secondary" onClick={() => navigate('/inicio')}>
              Ir al Inicio
            </button>
            {patientId && (
              <button className="btn-primary" onClick={() => navigate(`/patients/${patientId}/history`)}>
                Ver Historial del Paciente
              </button>
            )}
          </div>
        </div>
      </div>
    );
  }

return (
    <div className="consultation-manager-container">
      {/* CABECERA PRINCIPAL */}
      <header className="consultation-header">
        <div className="patient-summary">
          <button className="btn-back" onClick={() => navigate('/inicio')}>
            <span className="material-icons-outlined">arrow_back</span>
          </button>
          <div className="info">
            <h1>
              {(appointment?.patient || appointment?.Patient)?.firstName || (appointment?.patient || appointment?.Patient)?.FirstName}{' '}
              {(appointment?.patient || appointment?.Patient)?.lastName || (appointment?.patient || appointment?.Patient)?.LastName}
            </h1>
            <p>
              <strong>Motivo de Consulta:</strong> {appointment?.reason || appointment?.Reason}
              <span className="divider">|</span>
              <strong>Teléfono:</strong> {patientForm.phone || '—'}
            </p>
          </div>
        </div>
        <div className="header-actions">
          <button className="btn-outline-danger" onClick={() => navigate('/inicio')}>
            Pausar / Salir
          </button>
        </div>
      </header>

      {/* SELECTORES DE MODO (PESTAÑAS UNIFICADAS) */}
      <div className="mode-selector">
        <button 
          className={`tab-btn ${activeTab === 'generalData' ? 'active' : ''}`}
          onClick={() => setActiveTab('generalData')}
        >
          <span className="material-icons-outlined">person</span> 1. Datos Generales
        </button>
        <button 
          className={`tab-btn ${activeTab === 'medicalHistory' ? 'active' : ''}`}
          onClick={() => setActiveTab('medicalHistory')}
        >
          <span className="material-icons-outlined">folder_shared</span> 2. Antecedentes
        </button>
      </div>

      {/* WORKSPACE CONTENEDOR FLEX */}
      <div className="consultation-workspace-layout">
        
        {/* PANEL IZQUIERDO: FORMULARIOS */}
        <main className="consultation-form-panel">
          
          {/* TARJETA DINÁMICA SEGÚN PESTAÑA ACTIVA */}
          <section className="profile-card-section">
            <div className="tab-content">
              {activeTab === 'generalData' ? (
                <div className="form-grid">
                  <div className="form-group">
                    <label>Nombre(s)</label>
                    <input 
                      type="text" 
                      value={patientForm.firstName} 
                      onChange={e => setPatientFormData({...patientForm, firstName: e.target.value})} 
                    />
                  </div>
                  <div className="form-group">
                    <label>Apellidos</label>
                    <input 
                      type="text" 
                      value={patientForm.lastName} 
                      onChange={e => setPatientFormData({...patientForm, lastName: e.target.value})} 
                    />
                  </div>
                  <div className="form-group">
                    <label>Fecha de Nacimiento</label>
                    <input 
                      type="date" 
                      value={patientForm.birthDate} 
                      onChange={e => setPatientFormData({...patientForm, birthDate: e.target.value})} 
                    />
                  </div>
                  <div className="form-group">
                    <label>Teléfono</label>
                    <input 
                      type="tel" 
                      value={patientForm.phone} 
                      onChange={e => setPatientFormData({...patientForm, phone: e.target.value})} 
                    />
                  </div>
                  <div className="form-group">
                    <label>Correo Electrónico</label>
                    <input 
                      type="email" 
                      placeholder="ejemplo@correo.com"
                      value={patientForm.email} 
                      onChange={e => setPatientFormData({...patientForm, email: e.target.value})} 
                    />
                  </div>
                </div>
              ) : (
                /* PANEL DE ANTECEDENTES REESTRUCTURADO */
                <div className="medical-history-sections">
                  {/* SUBSECCIÓN 1: ALERTAS Y ALERGIAS (CRÍTICO) */}
                  <div className="history-subsection critical-box">
                    <h4><span className="material-icons-outlined">gpp_maybe</span> Alertas Médicas Directas</h4>
                    <div className="form-grid">
                      <div className="form-group full-width">
                        <label>Alergias Conocidas</label>
                        <input 
                          type="text"
                          placeholder="Ej: Penicilina, mariscos, ninguna..."
                          value={patientForm.medicalHistory.allergies} 
                          onChange={e => setPatientFormData({
                            ...patientForm, 
                            medicalHistory: { ...patientForm.medicalHistory, allergies: e.target.value }
                          })} 
                        />
                      </div>
                      <div className="form-group full-width">
                        <label>Medicamentos Actuales / Tratamientos activos</label>
                        <input 
                          type="text"
                          placeholder="Ej: Metformina 850mg c/24h..."
                          value={patientForm.medicalHistory.current_medication} 
                          onChange={e => setPatientFormData({
                            ...patientForm, 
                            medicalHistory: { ...patientForm.medicalHistory, current_medication: e.target.value }
                          })} 
                        />
                      </div>
                    </div>
                  </div>

                  {/* SUBSECCIÓN 2: HISTORIAL CLÍNICO CLAVE */}
                  <div className="history-subsection">
                    <h4><span className="material-icons-outlined">medical_services</span> Antecedentes Patológicos e Intervenciones</h4>
                    <div className="form-grid">
                      <div className="form-group">
                        <label>Antecedentes Patológicos</label>
                        <textarea 
                          rows={2}
                          placeholder="Enfermedades crónicas, cardiovasculares, etc."
                          value={patientForm.medicalHistory.pathological_history} 
                          onChange={e => setPatientFormData({
                            ...patientForm, 
                            medicalHistory: { ...patientForm.medicalHistory, pathological_history: e.target.value }
                          })} 
                        />
                      </div>
                      <div className="form-group">
                        <label>Antecedentes Quirúrgicos y Traumas</label>
                        <textarea 
                          rows={2}
                          placeholder="Cirugías previas, hospitalizaciones, fracturas..."
                          value={patientForm.medicalHistory.surgical_history} 
                          onChange={e => setPatientFormData({
                            ...patientForm, 
                            medicalHistory: { ...patientForm.medicalHistory, surgical_history: e.target.value }
                          })} 
                        />
                      </div>
                    </div>
                  </div>

                  {/* SUBSECCIÓN 3: HERENCIA Y ENTORNO */}
                  <div className="history-subsection">
                    <h4><span className="material-icons-outlined">family_restroom</span> Antecedentes Heredofamiliares y Estilo de Vida</h4>
                    <div className="form-grid">
                      <div className="form-group">
                        <label>Heredofamiliares (Padres, Abuelos, Hermanos)</label>
                        <textarea 
                          rows={2}
                          placeholder="Diabetes, hipertensión, neoplasias en la familia..."
                          value={patientForm.medicalHistory.hereditaryHistory} 
                          onChange={e => setPatientFormData({
                            ...patientForm, 
                            medicalHistory: { ...patientForm.medicalHistory, hereditaryHistory: e.target.value }
                          })} 
                        />
                      </div>
                      <div className="form-group">
                        <label>Hábitos, Estilo de Vida y No Patológicos</label>
                        <textarea 
                          rows={2}
                          placeholder="Tabaquismo, alcohol, actividad física, alimentación..."
                          value={patientForm.medicalHistory.habitsLifestyle} 
                          onChange={e => setPatientFormData({
                            ...patientForm, 
                            medicalHistory: { ...patientForm.medicalHistory, habitsLifestyle: e.target.value }
                          })} 
                        />
                      </div>
                    </div>
                  </div>

                  {/* SUBSECCIÓN 4: GINECOOBSTÉRICOS */}
                  <div className="history-subsection">
                    <h4><span className="material-icons-outlined">female</span> Antecedentes Ginecoobstétricos (Si aplica)</h4>
                    <div className="form-grid">
                      <div className="form-group full-width">
                        <label>Registro Ginecoobstétrico</label>
                        <input 
                          type="text"
                          placeholder="Ej: Menarca: 12, FUM: 15/05/26, Ciclos: 28/5, G:1 P:1 C:0 A:0"
                          value={patientForm.medicalHistory.gynecoObstetric} 
                          onChange={e => setPatientFormData({
                            ...patientForm, 
                            medicalHistory: { ...patientForm.medicalHistory, gynecoObstetric: e.target.value }
                          })} 
                        />
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </section>

          {/* SECCIÓN DE DOCUMENTOS DE LA CITA */}
          <section className="profile-card-section">
            <div className="section-header-flex">
              <h3 className="section-title">Documentos de la Cita</h3>
              {isSyncingFiles && <span className="sync-badge">Sincronizando...</span>}
            </div>
            
            <div className="upload-options">
              <div className="upload-card" onClick={() => fileInputRef.current?.click()}>
                <input type="file" ref={fileInputRef} onChange={handleFileUpload} multiple style={{display:'none'}} />
                <span className="material-icons-outlined">computer</span>
                <p>Carga Local</p>
              </div>
              <div className="upload-card" onClick={toggleQR}>
                <span className="material-icons-outlined">qr_code_scanner</span>
                <p>{showQR ? 'Ocultar QR' : 'Solicitar al Paciente'}</p>
              </div>
            </div>

            {showQR && (
              <div className="qr-display">
                <img src={qrImageUrl} alt="QR de carga" />
                <p className="qr-desc">Pida al paciente que escanee este código para subir fotos desde su móvil.</p>
              </div>
            )}

            {uploadedFiles.length > 0 && (
              <div className="file-list">
                {uploadedFiles.map((file, idx) => (
                  <div key={idx} className={`file-item ${selectedSidebarFile?.url === file.url ? 'active-file' : ''}`}>
                    <div 
                      className="file-info" 
                      onClick={() => setSelectedSidebarFile(file)} 
                      style={{ cursor: 'pointer', flex: 1 }}
                      title="Haz clic para ver de forma lateral"
                    >
                      <span className="material-icons-outlined">
                        {file.type?.startsWith('image/') ? 'image' : 'picture_as_pdf'}
                      </span>
                      <span className="file-name">{file.name}</span>
                    </div>
                    <div className="file-actions">
                      <a 
                        href={file.url} 
                        target="_blank" 
                        rel="noreferrer" 
                        className="btn-icon"
                        title="Abrir en pestaña nueva"
                      >
                        <span className="material-icons-outlined">open_in_new</span>
                      </a>
                      <button 
                        type="button" 
                        className="btn-icon btn-danger" 
                        onClick={() => removeFile(file)}
                      >
                        <span className="material-icons-outlined">delete</span>
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>

          {/* NOTAS DE LA CONSULTA (EVOLUCIÓN DINÁMICA) */}
          <section className="form-card highlighted-section">
            <h3>
              <span className="material-icons-outlined">analytics</span>
              Notas de la Consulta (Evolución)
            </h3>
            <div className="form-grid">
              {sectionsConfig.map((section) => (
                <div className="form-group full-width" key={section.id}>
                  {/* Contenedor flexible para alinear título a la izquierda y checkbox a la derecha */}
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                    <label style={{ margin: 0 }}>
                      {section.label} {section.required && <span className="req-asterisk">*</span>}
                    </label>
                    
                    {/* NUEVO: Checkbox para incluir/excluir este apartado específico de la receta */}
                    <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '13px', color: '#005073', cursor: 'pointer', fontWeight: 500 }}>
                      <input 
                        type="checkbox" 
                        style={{ width: '16px', height: '16px', accentColor: '#005073', cursor: 'pointer' }}
                        checked={recipeSections[section.label] !== false} // Por defecto true si no está explícitamente en false
                        onChange={(e) => setRecipeSections({
                          ...recipeSections,
                          [section.label]: e.target.checked
                        })}
                      />
                      Incluir en Receta
                    </label>
                  </div>

                  <textarea 
                    rows={section.id === 'diagnostico' ? 2 : 4}
                    placeholder={section.placeholder}
                    value={dynamicNotes[section.label] || ''} 
                    onChange={(e) => setDynamicNotes({
                      ...dynamicNotes,
                      [section.label]: e.target.value
                    })}
                  />
                </div>
              ))}
            </div>
          </section>


          <button 
            type="button"
            className="btn-secondary"
            onClick={handleCreateRecipeClick}
            disabled={generatingRecipe || loading}
            style={{ backgroundColor: recipeGenerated ? '#e6fffa' : '', borderColor: recipeGenerated ? '#319795' : '' }}
          >
            <span className="material-icons-outlined">
              {recipeGenerated ? 'badge' : 'description'}
            </span>
            {generatingRecipe ? 'Generando...' : recipeGenerated ? 'Receta Guardada ✓' : '1. Generar Receta'}
          </button>

          {/* PASO 2: BOTÓN PARA IMPRIMIR (Sólo se habilita si ya fue generada) */}
          <button 
            type="button"
            className="btn-secondary"
            onClick={handlePrintRecipeClick}
            disabled={!recipeGenerated || loading}
          >
            <span className="material-icons-outlined">print</span>
            2. Imprimir Receta
          </button>
        </main>

        {/* PANEL DERECHO: VISOR DE IMÁGENES / PDFS DE FORMA LATERAL */}
        {selectedSidebarFile && selectedSidebarFile.url && (
          <aside 
            className="consultation-sidebar-preview" 
            style={{ width: `${sidebarWidth}px` }}
          >
            {/* Barra divisoria para arrastrar */}
            <div className="sidebar-resizer" onMouseDown={startResizing} />

            <div className="sidebar-preview-content">
              <div className="sidebar-preview-header">
                <div className="title-area">
                  <span className="material-icons-outlined">visibility</span>
                  <span className="file-name-title" title={selectedSidebarFile.name}>
                    {selectedSidebarFile.name}
                  </span>
                </div>
                <div className="header-actions">
                  <a href={selectedSidebarFile.url} target="_blank" rel="noreferrer" className="btn-icon" title="Abrir en pantalla completa">
                    <span className="material-icons-outlined">open_in_new</span>
                  </a>
                  <button onClick={() => setSelectedSidebarFile(null)} className="btn-close">
                    <span className="material-icons-outlined">close</span>
                  </button>
                </div>
              </div>

              <div className="sidebar-preview-body">
                {selectedSidebarFile.type?.startsWith('image/') ? (
                  <img 
                    src={selectedSidebarFile.url} 
                    alt="Vista previa" 
                    className="img-fluid-preview" 
                  />
                ) : (
                  <iframe 
                    src={selectedSidebarFile.url} 
                    title="Documento de Consulta" 
                    className="iframe-pdf-preview"
                  />
                )}
              </div>
            </div>
          </aside>
        )}
      </div>

      {/* TOAST DE BORRADOR AUTOMÁTICO */}
      {showAutosaveToast && (
        <div className="autosave-toast">
          <span className="material-icons-outlined">cloud_done</span>
          <span>Borrador guardado automáticamente</span>
        </div>
      )}
    </div>
  );
};

