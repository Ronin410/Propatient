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

const baseurl = "http://localhost:8095"

type FormSection = 'generalData' | 'medicalHistory';

export const ConsultationManager: React.FC = () => {
  const { appointmentId } = useParams<{ appointmentId: string }>();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  
  const [appointment, setAppointment] = useState<Appointment | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // --- REFERENCIA PARA CONTROL DE CAMBIOS (isDirty) ---
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

  const loadConsultation = useCallback(async () => {
    try {
      setLoading(true);
      const response = await api.get(`/appointments/${appointmentId}`);
      const data = response.data;
      setAppointment(data);

      const p = data.patient || data.Patient;
      const h = p?.MedicalHistory || p?.medicalHistory;

      const initialPatientData = {
        firstName: p?.firstName || p?.FirstName || '',
        lastName: p?.lastName || p?.LastName || '',
        birthDate: (p?.birthDate || p?.BirthDate)?.split('T')[0] || '',
        gender: p?.gender || p?.Gender || '',
        phone: p?.phone || p?.Phone || '',
        email: p?.email || p?.Email || '',
        address: p?.address || p?.Address || '',
        medicalHistory: {
          // Soportamos tanto la nomenclatura directa de Go como la snake_case tradicional
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

      lastSavedDataRef.current = JSON.stringify({ 
        patientForm: initialPatientData, 
        consultationNotes: { subjective: '', objective: '', diagnosis: '', treatmentPlan: '' } 
      });

      if (data.documents) {
        mapServerFiles(data.documents);
      }

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
        
        // Mapeamos lo que viene del servidor de forma normal
        const mappedServerFiles: AppointmentFile[] = serverDocs.map((doc: any) => ({
          id: doc.id,
          name: doc.filename,
          type: doc.fileType,
          size: doc.data ? Math.round(doc.data.length * 0.75) : 0,
          url: `${baseurl}${doc.file_path}`,
          isServerFile: true
        }));

        // NUEVO: Sincronización inteligente sin destruir tus cambios locales
        setUploadedFiles(prevFiles => {
          const localOnlyFiles = prevFiles.filter(f => !f.isServerFile);
          const localNamesMap = new Map(prevFiles.map(f => [f.id || f.url, f.name]));

          const combinedServerFiles = mappedServerFiles.map(sf => {
            const key = sf.id || sf.url;
            return {
              ...sf,
              name: localNamesMap.has(key) ? localNamesMap.get(key)! : sf.name // Respeta el nombre editado
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

// --- EFFECT: AUTOGUARDADO EN LOCALSTORAGE (CADA 30 SEGUNDOS) ---
  useEffect(() => {
    if (loading || !appointmentId) return;

    const saveDraft = () => {
      const currentData = {
        patientForm,
        consultationNotes,
        uploadedFiles, // ◄ PUNTO CLAVE: Agregado para que se incluya en la copia de seguridad
      };
      
      const currentDataStr = JSON.stringify(currentData);
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

    const interval = setInterval(saveDraft, 30000);
    return () => clearInterval(interval);
    // ◄ IMPORTANTE: Dejamos el arreglo de dependencias exactamente igual al tuyo original
  }, [loading, appointmentId, patientForm, consultationNotes]);

const checkAndRestoreDraft = useCallback(() => {
    const savedDraft = localStorage.getItem(`consultation_draft_${appointmentId}`);
    if (savedDraft) {
      const draft = JSON.parse(savedDraft);
      if (window.confirm(`Se encontró un borrador guardado automáticamente el ${new Date(draft.timestamp).toLocaleString()}. ¿Desea recuperarlo?`)) {
        setPatientFormData(draft.patientForm);
        setConsultationNotes(draft.consultationNotes);
        
        // NUEVO: Si la copia del localstorage tiene archivos guardados, los reinyecta
        if (draft.uploadedFiles && draft.uploadedFiles.length > 0) {
          setUploadedFiles(draft.uploadedFiles);
        }
        
        lastSavedDataRef.current = JSON.stringify({ patientForm: draft.patientForm, consultationNotes: draft.consultationNotes });
      }
    }
  }, [appointmentId]);

  const handleFinalize = async () => {
    if (!consultationNotes.diagnosis || !consultationNotes.treatmentPlan) {
      alert("El diagnóstico y el plan de tratamiento son obligatorios para finalizar la consulta.");
      return;
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

      const localFiles = uploadedFiles.filter(f => !f.isServerFile && f.originalFile);
      if (localFiles.length > 0) {
        const formData = new FormData();
        localFiles.forEach(f => formData.append('files', f.originalFile!));
        formData.append('isPrescription', 'false');
        await api.post(`/appointments/${appointmentId}/upload-document`, formData, {
          headers: { 'Content-Type': 'multipart/form-data' }
        });
      }

      const appointmentUpdatePayload = {
        ...appointment,
        status: 'COMPLETED',
        diagnosis: consultationNotes.diagnosis,
        treatmentPlan: consultationNotes.treatmentPlan,
        notes: `SUBJETIVO: ${consultationNotes.subjective}\nOBJECTIVO: ${consultationNotes.objective}`
      };
      await api.put(`/appointments/${appointmentId}`, appointmentUpdatePayload);

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
    const interval = setInterval(refreshFiles, 20000);
    return () => clearInterval(interval);
  }, [appointmentId, loadConsultation, checkAndRestoreDraft]);

  if (loading) return <div className="consultation-manager-container loading-state"><div className="spinner"></div><p>Iniciando consulta médica...</p></div>;
  if (error || !appointment) return <div className="consultation-manager-container error-state"><p>{error}</p></div>;

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
              {(appointment.patient || appointment.Patient)?.firstName || (appointment.patient || appointment.Patient)?.FirstName}{' '}
              {(appointment.patient || appointment.Patient)?.lastName || (appointment.patient || appointment.Patient)?.LastName}
            </h1>
            <p>
              <strong>Motivo de Consulta:</strong> {appointment.reason}
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

      {/* CONTENIDO EN ANCHO COMPLETO (Se removió la columna lateral sobrante) */}
      <div className="consultation-content-fullwidth">
        <main className="consultation-main-panel">
          
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
                  {/* CAMBIO CORRECTO: Campo de Correo Electrónico Editable */}
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

                  {/* SUBSECCIÓN 2: HISTORIAL CLÍNICO PRINCIPAL */}
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
                          value={patientForm.medicalHistory.gynecoObstetric || patientForm.medicalHistory.gynecoObstetric} 
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
              <ul className="file-list">
                {uploadedFiles.map((file, idx) => (
                  <li key={idx} className="file-item">
                    <div className="file-info">
                      <span className="material-icons-outlined file-ic">
                        {file.type.includes('image') ? 'image' : 'description'}
                      </span>
                      <input
                        type="text"
                        className="file-rename-input"
                        value={file.name}
                        onChange={(e) => handleRenameFile(idx, e.target.value)}
                        style={{
                          border: 'none',
                          borderBottom: '1px dashed #ccc',
                          background: 'transparent',
                          padding: '2px 4px',
                          fontSize: '0.9rem',
                          fontWeight: 500,
                          width: '100%',
                          color: '#002d42'
                        }}
                      />
                                  
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

          {/* NOTAS DE LA CONSULTA (EVOLUCIÓN) */}
          <section className="profile-card-section highlighted-section">
            <h3 className="section-title">Notas de la Consulta</h3>
            <div className="form-grid">
              <div className="form-group full-width">
                <label>Padecimiento Actual (Subjetivo)</label>
                <textarea 
                  rows={3}
                  placeholder="Lo que el paciente refiere..."
                  value={consultationNotes.subjective}
                  onChange={e => setConsultationNotes({...consultationNotes, subjective: e.target.value})}
                />
              </div>
              <div className="form-group full-width">
                <label>Diagnóstico</label>
                <input 
                  type="text" 
                  placeholder="Impresión diagnóstica..."
                  value={consultationNotes.diagnosis}
                  onChange={e => setConsultationNotes({...consultationNotes, diagnosis: e.target.value})}
                />
              </div>
              <div className="form-group full-width">
                <label>Plan y Tratamiento</label>
                <textarea 
                  rows={5}
                  placeholder="Receta e indicaciones..."
                  value={consultationNotes.treatmentPlan}
                  onChange={e => setConsultationNotes({...consultationNotes, treatmentPlan: e.target.value})}
                />
              </div>
            </div>
          </section>

          {/* ACCIÓN DE CIERRE */}
          <div className="consultation-actions">
            <button className="btn-primary-lg" onClick={handleFinalize}>
              <span className="material-icons-outlined">assignment_turned_in</span> Finalizar Consulta y Generar Receta
            </button>
          </div>
        </main>
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
