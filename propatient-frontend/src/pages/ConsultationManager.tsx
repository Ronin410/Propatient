import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
// import pdfMake from 'pdfmake';
// import pdfFonts from 'pdfmake/build/vfs_fonts';
import type { Patient, Appointment, MedicalHistory, ConsultationNotes } from '../types';
import './ConsultationManager.scss';
import api from '../api/axios';
interface AppointmentFile {
  id?: number;
  name: string;
  type: string;
  size: number;
  url: string;
  originalFile?: File;
  isServerFile?: boolean;
}

// Configuración de fuentes para pdfmake
// pdfMake.vfs = pdfFonts.pdfMake.vfs;

type FormSection = 'generalData' | 'medicalHistory';

export const ConsultationManager: React.FC = () => {
  const { appointmentId } = useParams<{ appointmentId: string }>();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  
  const [appointment, setAppointment] = useState<Appointment | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // --- REFERENCIA PARA CONTROL DE CAMBIOS (isDirty) ---
  // Guardamos la representación en cadena de los datos para una comparación profunda simple
  const lastSavedDataRef = useRef<string>("");

  // --- ESTADOS DEL FORMULARIO ---
  const [activeTab, setActiveTab] = useState<FormSection>('generalData');
  
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
      allergies: '',
      pathological_history: '',
      non_pathological_history: '',
      surgical_history: '',
      current_medication: ''
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

  const loadConsultation = useCallback(async () => {
    try {
      setLoading(true);
      const response = await api.get(`/appointments/${appointmentId}`); // Preload("Patient.MedicalHistory") en Go
      const data = response.data;
      setAppointment(data);

      // 1. Preparar los datos del formulario siguiendo la estructura exacta del estado local
      const p = data.patient || data.Patient;
      const initialPatientData = {
        firstName: p?.firstName || p?.FirstName || '',
        lastName: p?.lastName || p?.LastName || '',
        birthDate: (p?.birthDate || p?.BirthDate)?.split('T')[0] || '',
        gender: p?.gender || '',
        phone: p?.phone || '',
        email: p?.email || '',
        address: p?.address || '',
        medicalHistory: {
          allergies: p?.MedicalHistory?.allergies || '',
          pathological_history: p.MedicalHistory?.pathological_history || '',
          non_pathological_history: p.MedicalHistory?.non_pathological_history || '',
          surgical_history: p.MedicalHistory?.surgical_history || '',
          current_medication: p.MedicalHistory?.current_medication || ''
        }
      };

      // Actualizar estado del formulario
      setPatientFormData(initialPatientData);

      // Inicializar la referencia con los datos cargados para que el primer autoguardado no sea redundante
      lastSavedDataRef.current = JSON.stringify({ 
        patientForm: initialPatientData, 
        consultationNotes: { subjective: '', objective: '', diagnosis: '', treatmentPlan: '' } 
      });

      if (data.documents) {
        mapServerFiles(data.documents);
      }

      // Lógica de Angular: Si está PENDING, la pasamos a IN_COURSE
      if (data.status === 'PENDING') {
        await api.put(`/appointments/${appointmentId}`, { 
          ...data, 
          status: 'IN_COURSE' 
        });
      }
    } catch (err) {
      console.error("Error cargando consulta:", err);
      setError("No se pudo cargar la información de la consulta.");
    } finally {
      setLoading(false);
    }
  }, [appointmentId]);

  const mapServerFiles = (docs: any[]) => {
    const files: AppointmentFile[] = docs.map(doc => ({
      id: doc.id,
      name: doc.fileName,
      type: doc.fileType,
      size: doc.data ? Math.round(doc.data.length * 0.75) : 0,
      url: `data:${doc.fileType};base64,${doc.data}`,
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
        mapServerFiles(response.data.documents);
      }
    } catch (error) {
      console.error("Error sincronizando archivos:", error);
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

  const toggleQR = () => {
    if (!showQR) {
      const uploadUrl = `${window.location.origin}/public-upload/${appointmentId}`;
      setQrImageUrl(`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(uploadUrl)}`);
    }
    setShowQR(!showQR);
  };

  // --- LÓGICA DE AUTOGUARDADO (DRAFT) ---
  useEffect(() => {
    if (loading || !appointmentId) return;

    const saveDraft = () => {
      const currentData = {
        patientForm,
        consultationNotes,
      };
      
      const currentDataStr = JSON.stringify(currentData);

      // --- COMPROBACIÓN isDirty ---
      // Si los datos actuales son idénticos a los últimos guardados, cancelamos la operación
      if (currentDataStr === lastSavedDataRef.current) return;

      const draftData = {
        ...currentData,
        timestamp: new Date().toISOString(),
      };
      localStorage.setItem(`consultation_draft_${appointmentId}`, JSON.stringify(draftData));

      // Actualizar la referencia del último guardado
      lastSavedDataRef.current = currentDataStr;

      // Disparar la notificación visual
      setShowAutosaveToast(true);
      setTimeout(() => setShowAutosaveToast(false), 3000);
    };

    const interval = setInterval(saveDraft, 30000); // 30 segundos
    return () => clearInterval(interval);
  }, [loading, appointmentId, patientForm, consultationNotes]);

  const checkAndRestoreDraft = useCallback(() => {
    const savedDraft = localStorage.getItem(`consultation_draft_${appointmentId}`);
    if (savedDraft) {
      const draft = JSON.parse(savedDraft);
      if (window.confirm(`Se encontró un borrador guardado automáticamente el ${new Date(draft.timestamp).toLocaleString()}. ¿Desea recuperarlo?`)) {
        setPatientFormData(draft.patientForm);
        setConsultationNotes(draft.consultationNotes);
        
        // Actualizar la referencia con los datos restaurados
        lastSavedDataRef.current = JSON.stringify({ patientForm: draft.patientForm, consultationNotes: draft.consultationNotes });
      }
    }
  }, [appointmentId]);

  const generatePrescriptionPDF = (mode: 'download' | 'preview' = 'download') => {
    // Comentado temporalmente por error de importación
    console.log("Generando PDF en modo:", mode);
    // const docDefinition = {
    //   content: [
    //     { text: 'RECETA MÉDICA', style: 'header', alignment: 'center' as const },
    //     { text: '\n\n' },
    //     {
    //       columns: [
    //         { text: `Paciente: ${patientForm.firstName} ${patientForm.lastName}`, bold: true },
    //         { text: `Fecha: ${new Date().toLocaleDateString()}`, alignment: 'right' as const }
    //       ]
    //     },
    //     { text: `Edad: ${appointment?.patient.birthDate ? (new Date().getFullYear() - new Date(appointment.patient.birthDate).getFullYear()) : 'N/A'} años` },
    //     { text: '\n' },
    //     { canvas: [{ type: 'line', x1: 0, y1: 5, x2: 515, y2: 5, lineWidth: 1 }] },
    //     { text: '\n' },
    //     { text: 'DIAGNÓSTICO:', style: 'subheader' },
    //     { text: consultationNotes.diagnosis || 'Sin diagnóstico registrado.' },
    //     { text: '\n' },
    //     { text: 'INDICACIONES Y TRATAMIENTO:', style: 'subheader' },
    //     { text: consultationNotes.treatmentPlan || 'Sin indicaciones registradas.' },
    //     { text: '\n\n\n\n' },
    //     { text: '__________________________', alignment: 'center' as const },
    //     { text: 'Firma del Médico', alignment: 'center' as const, fontSize: 10 }
    //   ],
    //   styles: {
    //     header: { fontSize: 20, bold: true, color: '#2c3e50' },
    //     subheader: { fontSize: 14, bold: true, margin: [0, 10, 0, 5] as [number, number, number, number], color: '#34495e' }
    //   }
    // };

    // if (mode === 'preview') {
    //   pdfMake.createPdf(docDefinition).open();
    // } else {
    //   pdfMake.createPdf(docDefinition).download(`Receta_${patientForm.lastName}_${appointmentId}.pdf`);
    // }
  };

  const handleFinalize = async () => {
    // 1. Validación básica
    if (!consultationNotes.diagnosis || !consultationNotes.treatmentPlan) {
      alert("El diagnóstico y el plan de tratamiento son obligatorios para finalizar la consulta.");
      return;
    }

    // Validar correo del paciente antes de actualizar su expediente
    if (patientForm.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(patientForm.email)) {
      alert("El correo electrónico del paciente no tiene un formato válido.");
      return;
    }

    const confirmClose = window.confirm("¿Está seguro de finalizar la consulta? Se generará la receta y se cerrará el expediente de esta cita.");
    if (!confirmClose) return;

    setLoading(true);
    try {
      // 2. Actualizar datos del paciente y su historial médico
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
          current_medication: patientForm.medicalHistory.current_medication
        }
      };
      
      const patientId = appointment?.patient?.id || appointment?.Patient?.id || (appointment?.patient as any)?.ID;
      await api.put(`/patients/${patientId}`, patientUpdatePayload);

      // 3. Subir archivos locales que aún no están en el servidor
      const localFiles = uploadedFiles.filter(f => !f.isServerFile && f.originalFile);
      if (localFiles.length > 0) {
        const formData = new FormData();
        localFiles.forEach(f => formData.append('files', f.originalFile!));
        formData.append('isPrescription', 'false');
        await api.post(`/appointments/${appointmentId}/documents`, formData, {
          headers: { 'Content-Type': 'multipart/form-data' }
        });
      }

      // 4. Actualizar estado de la cita a COMPLETED y guardar notas
      const appointmentUpdatePayload = {
        ...appointment,
        status: 'COMPLETED',
        diagnosis: consultationNotes.diagnosis,
        treatmentPlan: consultationNotes.treatmentPlan,
        notes: `SUBJETIVO: ${consultationNotes.subjective}\nOBJECTIVO: ${consultationNotes.objective}`
      };
      await api.put(`/appointments/${appointmentId}`, appointmentUpdatePayload);

      // 5. Generar PDF
      // generatePrescriptionPDF();

      // 6. Limpiar borrador tras éxito
      localStorage.removeItem(`consultation_draft_${appointmentId}`);

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
        checkAndRestoreDraft();
      }
    };
    init();
    const interval = setInterval(refreshFiles, 20000); // Sincronizar cada 20s
    return () => clearInterval(interval);
  }, [appointmentId, loadConsultation, checkAndRestoreDraft]);

  if (loading) return <div className="loading-state">Iniciando consulta médica...</div>;
  if (error || !appointment) return <div className="error-state">{error}</div>;

  return (
    <div className="consultation-manager-container">
      <header className="consultation-header">
        <div className="patient-summary">
          <button className="btn-back" onClick={() => navigate('/inicio')}>
            <span className="material-icons-outlined">arrow_back</span>
          </button>
          <div className="info">
            <h1>Consulta en Curso</h1>
            <p>
              <strong>Paciente:</strong> {
                (appointment.patient || appointment.Patient)?.firstName || 
                (appointment.patient || appointment.Patient)?.FirstName
              } {(appointment.patient || appointment.Patient)?.lastName || (appointment.patient || appointment.Patient)?.LastName} 
              <span className="divider">|</span>
              <strong>Motivo:</strong> {appointment.reason}
            </p>
          </div>
        </div>
        <div className="header-actions">
          <button className="btn-outline-danger" onClick={() => navigate('/inicio')}>
            Pausar / Salir
          </button>
        </div>
      </header>

      <div className="consultation-content-grid">
        <aside className="patient-sidebar">
          <div className="card">
            <h3>Historial Rápido</h3>
            <div className="quick-info">
              <p><strong>Teléfono:</strong> {appointment.patient?.phone}</p>
              <p><strong>Email:</strong> {appointment.patient?.email}</p>
            </div>
          </div>
        </aside>

        <main className="consultation-form-area">
          <section className="patient-data-section card">
            <nav className="form-tabs">
              <button 
                className={`tab-link ${activeTab === 'generalData' ? 'active' : ''}`}
                onClick={() => setActiveTab('generalData')}
              >
                1. Datos Generales
              </button>
              <button 
                className={`tab-link ${activeTab === 'medicalHistory' ? 'active' : ''}`}
                onClick={() => setActiveTab('medicalHistory')}
              >
                2. Antecedentes
              </button>
            </nav>

            <div className="tab-content">
              {activeTab === 'generalData' ? (
                <div className="form-grid">
                  <div className="form-group">
                    <label>Nombre(s)</label>
                    <input type="text" value={patientForm.firstName} onChange={e => setPatientFormData({...patientForm, firstName: e.target.value})} />
                  </div>
                  <div className="form-group">
                    <label>Apellidos</label>
                    <input type="text" value={patientForm.lastName} onChange={e => setPatientFormData({...patientForm, lastName: e.target.value})} />
                  </div>
                  <div className="form-group">
                    <label>Fecha de Nacimiento</label>
                    <input type="date" value={patientForm.birthDate} onChange={e => setPatientFormData({...patientForm, birthDate: e.target.value})} />
                  </div>
                  <div className="form-group">
                    <label>Teléfono</label>
                    <input type="tel" value={patientForm.phone} onChange={e => setPatientFormData({...patientForm, phone: e.target.value})} />
                  </div>
                </div>
              ) : (
                <div className="form-grid">
                  <div className="form-group full-width">
                    <label>Alergias</label>
                    <textarea 
                      value={patientForm.medicalHistory.allergies} 
                      onChange={e => setPatientFormData({
                        ...patientForm, 
                        medicalHistory: { ...patientForm.medicalHistory, allergies: e.target.value }
                      })} 
                    />
                  </div>
                  <div className="form-group full-width">
                    <label>Antecedentes Patológicos</label>
                    <textarea 
                      value={patientForm.medicalHistory.pathological_history} 
                      onChange={e => setPatientFormData({
                        ...patientForm, 
                        medicalHistory: { ...patientForm.medicalHistory, pathological_history: e.target.value }
                      })} 
                    />
                  </div>
                </div>
              )}
            </div>
          </section>

          <section className="document-upload-section card mt-4">
            <div className="section-header-flex">
              <h3>Documentos de la Cita</h3>
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
              <div className="qr-display card">
                <img src={qrImageUrl} alt="QR de carga" />
                <p className="text-muted small">Pida al paciente que escanee este código para subir fotos desde su móvil.</p>
              </div>
            )}

            {uploadedFiles.length > 0 && (
              <ul className="file-list mt-3">
                {uploadedFiles.map((file, idx) => (
                  <li key={idx} className="file-item">
                    <div className="file-info">
                      <span className="material-icons-outlined">
                        {file.type.includes('image') ? 'image' : 'description'}
                      </span>
                      <span className="file-name">{file.name}</span>
                      {file.isServerFile && <span className="server-label">Paciente</span>}
                    </div>
                    <div className="file-actions">
                      <button className="btn-icon" onClick={() => window.open(file.url, '_blank')}>
                        <span className="material-icons-outlined">visibility</span>
                      </button>
                      <button className="btn-icon btn-danger" onClick={() => removeFile(file)}>
                        <span className="material-icons-outlined">delete</span>
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="consultation-notes-section card mt-4">
            <h3>Notas de la Consulta</h3>
            <div className="form-group">
              <label>Padecimiento Actual (Subjetivo)</label>
              <textarea 
                placeholder="Lo que el paciente refiere..."
                value={consultationNotes.subjective}
                onChange={e => setConsultationNotes({...consultationNotes, subjective: e.target.value})}
              />
            </div>
            <div className="form-group">
              <label>Diagnóstico</label>
              <input 
                type="text" 
                placeholder="Impresión diagnóstica..."
                value={consultationNotes.diagnosis}
                onChange={e => setConsultationNotes({...consultationNotes, diagnosis: e.target.value})}
              />
            </div>
            <div className="form-group">
              <label>Plan y Tratamiento</label>
              <textarea 
                placeholder="Receta e indicaciones..."
                value={consultationNotes.treatmentPlan}
                onChange={e => setConsultationNotes({...consultationNotes, treatmentPlan: e.target.value})}
              />
            </div>
          </section>

          <div className="form-actions sticky-bottom">
            {/* <button 
              className="btn-outline-secondary btn-lg" 
              style={{ marginRight: '1rem' }}
              onClick={() => generatePrescriptionPDF('preview')}
              disabled={!consultationNotes.diagnosis || !consultationNotes.treatmentPlan}
            >
              <span className="material-icons-outlined">visibility</span>
              Vista Previa Receta
            </button> */}
            <button className="btn-primary btn-lg" onClick={handleFinalize}>
              Finalizar Consulta y Generar Receta
            </button>
          </div>
        </main>
      </div>

      {/* Notificación de Autoguardado */}
      {showAutosaveToast && (
        <div className="autosave-toast">
          <span className="material-icons-outlined">cloud_done</span>
          <span>Borrador guardado automáticamente</span>
        </div>
      )}
    </div>
  );
};