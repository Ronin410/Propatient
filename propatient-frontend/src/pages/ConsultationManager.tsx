import React, { useEffect, useRef, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import './ConsultationManager.scss';
import { useConsultation, type AppointmentFile } from '../hooks/useConsultation';
import { toAbsoluteFileUrl } from '../utils/fileUrl';
import { sanitizePhoneInput } from '../utils/phoneInput';
import api from '../api/axios';
import { getErrorMessage } from '../utils/errorMessage';
import {
  ChecklistField, HabitsLifestyleField, GynecoObstetricField,
  ALLERGY_OPTIONS, PATHOLOGICAL_OPTIONS, SURGICAL_OPTIONS, HEREDITARY_OPTIONS,
} from '../components/MedicalHistoryFields';

type FormSection = 'generalData' | 'medicalHistory';

interface NoteHistoryEntry {
  id: number;
  createdAt: string;
  previousDiagnosis: string;
  previousDiagnosisCode: string;
  previousTreatmentPlan: string;
  previousNotes: string;
  changedByRole: 'MEDICO' | 'STAFF';
  changedByName: string;
}

interface Cie10Result {
  code: string;
  name: string;
  chapter: string;
}

export const ConsultationManager: React.FC = () => {
  const { appointmentId } = useParams<{ appointmentId: string }>();
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const {
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
    hasSectionsForRecipe,
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
  } = useConsultation(appointmentId);

  // --- Estado puramente de UI (no es dominio de la consulta) ---
  const [activeTab, setActiveTab] = useState<FormSection>('generalData');
  const [selectedSidebarFile, setSelectedSidebarFile] = useState<AppointmentFile | null>(null);
  const [sidebarWidth, setSidebarWidth] = useState<number>(380);
  const isResizingRef = useRef<boolean>(false);
  const [showQR, setShowQR] = useState(false);
  const [qrImageUrl, setQrImageUrl] = useState('');
  const [loadingQR, setLoadingQR] = useState(false);
  const [qrError, setQrError] = useState<string | null>(null);

  const [showNoteHistory, setShowNoteHistory] = useState(false);
  const [noteHistory, setNoteHistory] = useState<NoteHistoryEntry[] | null>(null);
  const [noteHistoryLoading, setNoteHistoryLoading] = useState(false);

  // --- Buscador de diagnóstico CIE-10 (catálogo oficial DGIS) ---
  const [cie10SearchTerm, setCie10SearchTerm] = useState('');
  const [cie10Results, setCie10Results] = useState<Cie10Result[]>([]);
  const [cie10Searching, setCie10Searching] = useState(false);
  const [cie10ShowResults, setCie10ShowResults] = useState(false);

  useEffect(() => {
    if (!cie10SearchTerm.trim() || cie10SearchTerm.trim().length < 2) {
      setCie10Results([]);
      return;
    }
    const delayDebounce = setTimeout(async () => {
      setCie10Searching(true);
      try {
        const res = await api.get(`/utils/cie10?q=${encodeURIComponent(cie10SearchTerm.trim())}`);
        setCie10Results(res.data || []);
      } catch {
        setCie10Results([]);
      } finally {
        setCie10Searching(false);
      }
    }, 350);
    return () => clearTimeout(delayDebounce);
  }, [cie10SearchTerm]);

  const selectCie10Result = (result: Cie10Result) => {
    setDiagnosisCode(result.code);
    setDiagnosisCodeLabel(result.name);
    setCie10SearchTerm('');
    setCie10Results([]);
    setCie10ShowResults(false);
  };

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizingRef.current) return;
      const newWidth = window.innerWidth - e.clientX;
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
    document.body.style.userSelect = 'none';
  };

  const toggleQR = async () => {
    if (showQR) {
      setShowQR(false);
      return;
    }
    // El link/QR usa un token opaco generado por el backend (ver
    // GetAppointmentUploadLink), no el ID crudo de la cita: así quien lo
    // intercepte solo puede subir documentos a ESTA cita, no adivinar
    // otras. Se pide cada vez que se abre (el backend reutiliza el mismo
    // token mientras no haya vencido, así que sigue siendo el mismo QR).
    setShowQR(true);
    setLoadingQR(true);
    setQrError(null);
    try {
      const res = await api.get(`/appointments/${appointmentId}/upload-link`);
      const uploadUrl = res.data.uploadUrl as string;
      setQrImageUrl(`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(uploadUrl)}`);
    } catch (err) {
      setQrError(getErrorMessage(err, 'No se pudo generar el link para el paciente.'));
    } finally {
      setLoadingQR(false);
    }
  };

  const toggleNoteHistory = async () => {
    const next = !showNoteHistory;
    setShowNoteHistory(next);
    if (next && noteHistory === null) {
      setNoteHistoryLoading(true);
      try {
        const res = await api.get(`/appointments/${appointmentId}/note-history`);
        setNoteHistory(res.data || []);
      } catch {
        setNoteHistory([]);
      } finally {
        setNoteHistoryLoading(false);
      }
    }
  };

  if (loading) {
    return (
      <div className="consultation-manager-container loading-state">
        <div className="spinner"></div>
        <p>Iniciando consulta médica...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="consultation-error">
        <span className="material-icons-outlined">error_outline</span>
        <p>{error}</p>
        <button className="btn-primary" onClick={() => navigate('/inicio')}>Volver al Inicio</button>
      </div>
    );
  }

  const patientId = appointment?.patient?.id;
  // En modo lectura, "volver" debe regresar al historial del paciente (de
  // donde se llegó vía "Ver Consulta"), no al dashboard.
  const goBackTarget = isCompleted && patientId ? `/pacientes/${patientId}` : '/inicio';

  return (
    <div className="consultation-manager-container">
      {/* CABECERA PRINCIPAL */}
      <header className="consultation-header">
        <div className="patient-summary">
          <button className="btn-back" onClick={() => navigate(goBackTarget)}>
            <span className="material-icons-outlined">arrow_back</span>
          </button>
          <div className="info">
            <h1>
              {appointment?.patient?.firstName}{' '}
              {appointment?.patient?.lastName}
            </h1>
            <p>
              <strong>Motivo de Consulta:</strong> {appointment?.reason}
              <span className="divider">|</span>
              <strong>Teléfono:</strong> {patientForm.phone || '—'}
            </p>
          </div>
        </div>
        <div className="header-actions">
          <button className="btn-outline-sm" onClick={toggleNoteHistory}>
            {showNoteHistory ? 'Ocultar historial' : 'Historial de versiones'}
          </button>
          <button className="btn-outline-danger" onClick={() => navigate(goBackTarget)}>
            {isCompleted ? 'Volver' : 'Pausar / Salir'}
          </button>
        </div>
      </header>

      {showNoteHistory && (
        <div className="note-history-panel">
          <div className="note-history-panel-header">
            <span className="material-icons-outlined">history</span>
            <p>
              Versiones anteriores del diagnóstico, tratamiento y notas de esta cita — ninguna
              corrección borra el contenido previo, queda preservado aquí (NOM-024).
            </p>
          </div>
          {noteHistoryLoading ? (
            <p className="empty-msg">Cargando...</p>
          ) : !noteHistory || noteHistory.length === 0 ? (
            <p className="empty-msg">Todavía no hay versiones anteriores — esta cita no se ha corregido.</p>
          ) : (
            <ul className="note-history-list">
              {noteHistory.map((entry) => (
                <li key={entry.id} className="note-history-item">
                  <div className="note-history-meta">
                    <strong>{new Date(entry.createdAt).toLocaleString('es-MX')}</strong>
                    <span>
                      {entry.changedByName || 'Cuenta eliminada'} ({entry.changedByRole === 'STAFF' ? 'personal' : 'doctor'})
                    </span>
                  </div>
                  {entry.previousDiagnosis && (
                    <p><strong>Diagnóstico anterior:</strong> {entry.previousDiagnosis}</p>
                  )}
                  {entry.previousDiagnosisCode && (
                    <p><strong>Código CIE-10 anterior:</strong> {entry.previousDiagnosisCode}</p>
                  )}
                  {entry.previousTreatmentPlan && (
                    <p><strong>Tratamiento anterior:</strong> {entry.previousTreatmentPlan}</p>
                  )}
                  {entry.previousNotes && (
                    <p><strong>Notas anteriores:</strong> {entry.previousNotes}</p>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {isCompleted && (
        <div className="readonly-banner">
          <span className="material-icons-outlined">visibility</span>
          <span>
            Esta consulta ya fue finalizada. Estás viendo el expediente en modo solo lectura, sin poder modificarlo.
          </span>
          {patientId && (
            <button className="btn-text" onClick={() => navigate(`/pacientes/${patientId}`)}>
              Ver Historial del Paciente
            </button>
          )}
        </div>
      )}

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
                <fieldset disabled={isCompleted} className="form-grid fieldset-plain">
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
                      onChange={e => setPatientFormData({...patientForm, phone: sanitizePhoneInput(e.target.value)})}
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
                </fieldset>
              ) : (
                /* PANEL DE ANTECEDENTES REESTRUCTURADO — chips/booleanos +
                   texto libre en vez de solo texto libre, ver
                   MedicalHistoryFields.tsx. key=appointmentId fuerza que
                   cada campo reinicie su selección al cambiar de cita, en
                   vez de arrastrar la selección de la cita anterior. */
                <fieldset disabled={isCompleted} className="medical-history-sections fieldset-plain">
                  {/* SUBSECCIÓN 1: ALERTAS Y ALERGIAS (CRÍTICO) */}
                  <div className="history-subsection critical-box">
                    <h4><span className="material-icons-outlined">gpp_maybe</span> Alertas Médicas Directas</h4>
                    <div className="form-grid">
                      <div className="form-group full-width">
                        <label>Alergias Conocidas</label>
                        <ChecklistField
                          key={`allergies-${appointmentId}`}
                          value={patientForm.medicalHistory.allergies}
                          options={ALLERGY_OPTIONS}
                          noneLabel="Sin alergias conocidas"
                          otherPlaceholder="Otra alergia u observación..."
                          disabled={isCompleted}
                          onChange={next => setPatientFormData({
                            ...patientForm,
                            medicalHistory: { ...patientForm.medicalHistory, allergies: next }
                          })}
                        />
                      </div>
                      <div className="form-group full-width">
                        <label>Medicamentos Actuales / Tratamientos activos</label>
                        <ChecklistField
                          key={`medication-${appointmentId}`}
                          value={patientForm.medicalHistory.current_medication}
                          noneLabel="No toma medicamentos actualmente"
                          otherLabel="Medicamento, dosis y frecuencia"
                          otherPlaceholder="Ej: Metformina 850mg c/24h..."
                          disabled={isCompleted}
                          onChange={next => setPatientFormData({
                            ...patientForm,
                            medicalHistory: { ...patientForm.medicalHistory, current_medication: next }
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
                        <ChecklistField
                          key={`pathological-${appointmentId}`}
                          value={patientForm.medicalHistory.pathological_history}
                          options={PATHOLOGICAL_OPTIONS}
                          noneLabel="Sin antecedentes patológicos"
                          otherPlaceholder="Otra enfermedad crónica u observación..."
                          disabled={isCompleted}
                          onChange={next => setPatientFormData({
                            ...patientForm,
                            medicalHistory: { ...patientForm.medicalHistory, pathological_history: next }
                          })}
                        />
                      </div>
                      <div className="form-group">
                        <label>Antecedentes Quirúrgicos y Traumas</label>
                        <ChecklistField
                          key={`surgical-${appointmentId}`}
                          value={patientForm.medicalHistory.surgical_history}
                          options={SURGICAL_OPTIONS}
                          noneLabel="Sin cirugías ni traumatismos previos"
                          otherPlaceholder="Otra cirugía, hospitalización o fractura..."
                          disabled={isCompleted}
                          onChange={next => setPatientFormData({
                            ...patientForm,
                            medicalHistory: { ...patientForm.medicalHistory, surgical_history: next }
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
                        <ChecklistField
                          key={`hereditary-${appointmentId}`}
                          value={patientForm.medicalHistory.hereditaryHistory}
                          options={HEREDITARY_OPTIONS}
                          noneLabel="Sin antecedentes heredofamiliares relevantes"
                          otherLabel="Quién / detalles"
                          otherPlaceholder="Ej: Madre - diabetes, Abuelo paterno - cáncer..."
                          disabled={isCompleted}
                          onChange={next => setPatientFormData({
                            ...patientForm,
                            medicalHistory: { ...patientForm.medicalHistory, hereditaryHistory: next }
                          })}
                        />
                      </div>
                      <div className="form-group">
                        <label>Hábitos, Estilo de Vida y No Patológicos</label>
                        <HabitsLifestyleField
                          key={`habits-${appointmentId}`}
                          value={patientForm.medicalHistory.habitsLifestyle}
                          disabled={isCompleted}
                          onChange={next => setPatientFormData({
                            ...patientForm,
                            medicalHistory: { ...patientForm.medicalHistory, habitsLifestyle: next }
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
                        <GynecoObstetricField
                          key={`gyneco-${appointmentId}`}
                          value={patientForm.medicalHistory.gynecoObstetric}
                          disabled={isCompleted}
                          onChange={next => setPatientFormData({
                            ...patientForm,
                            medicalHistory: { ...patientForm.medicalHistory, gynecoObstetric: next }
                          })}
                        />
                      </div>
                    </div>
                  </div>
                </fieldset>
              )}
            </div>
          </section>

          {/* SECCIÓN DE DOCUMENTOS DE LA CITA */}
          <section className="profile-card-section">
            <div className="section-header-flex">
              <h3 className="section-title">Documentos de la Cita</h3>
              {isSyncingFiles && <span className="sync-badge">Sincronizando...</span>}
            </div>

            {!isCompleted && (
              <div className="upload-options">
                <div className="upload-card" onClick={() => fileInputRef.current?.click()}>
                  <input
                    type="file"
                    ref={fileInputRef}
                    onChange={handleFileUpload}
                    multiple
                    accept="image/*,application/pdf,.pdf,.doc,.docx,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
                    style={{display:'none'}}
                  />
                  <span className="material-icons-outlined">computer</span>
                  <p>Carga Local</p>
                </div>
                <div className="upload-card" onClick={toggleQR}>
                  <span className="material-icons-outlined">qr_code_scanner</span>
                  <p>{showQR ? 'Ocultar QR' : 'Solicitar al Paciente'}</p>
                </div>
              </div>
            )}

            {showQR && (
              <div className="qr-display">
                {loadingQR ? (
                  <p className="qr-desc">Generando link...</p>
                ) : qrError ? (
                  <p className="qr-desc qr-error">{qrError}</p>
                ) : (
                  <>
                    <img src={qrImageUrl} alt="QR de carga" />
                    <p className="qr-desc">Pida al paciente que escanee este código para subir sus estudios desde su móvil, o mándele el link por WhatsApp.</p>
                  </>
                )}
              </div>
            )}

            {uploadedFiles.length > 0 && (
              <div className="file-list">
                {uploadedFiles.map((file, idx) => (
                  <div key={idx} className={`file-item ${selectedSidebarFile?.url === file.url ? 'active-file' : ''}`}>
                    <div className="file-info" style={{ flex: 1 }}>
                      <span className="material-icons-outlined">
                        {file.type?.startsWith('image/') ? 'image' : 'picture_as_pdf'}
                      </span>
                      <span className="file-name">{file.name}</span>
                    </div>
                    <div className="file-actions">
                      <button
                        type="button"
                        className={`btn-icon ${selectedSidebarFile?.url === file.url ? 'btn-icon-active' : ''}`}
                        onClick={() => setSelectedSidebarFile(file)}
                        title="Ver al lado, sin salir de la cita"
                      >
                        <span className="material-icons-outlined">visibility</span>
                      </button>
                      <a
                        href={file.url}
                        target="_blank"
                        rel="noreferrer"
                        className="btn-icon"
                        title="Abrir en pestaña nueva"
                      >
                        <span className="material-icons-outlined">open_in_new</span>
                      </a>
                      {!isCompleted && (
                        <button
                          type="button"
                          className="btn-icon btn-danger"
                          onClick={() => removeFile(file)}
                          title="Eliminar"
                        >
                          <span className="material-icons-outlined">delete</span>
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
            {isCompleted && uploadedFiles.length === 0 && (
              <p className="empty-msg">No se adjuntaron archivos en esta consulta.</p>
            )}
          </section>

          {/* DIAGNÓSTICO ESTRUCTURADO (CATÁLOGO CIE-10) */}
          <section className="form-card cie10-card">
            <h3>
              <span className="material-icons-outlined">medical_information</span>
              Diagnóstico (CIE-10)
            </h3>
            <p className="cie10-subtitle">
              Opcional — busca por código o por nombre para asociar un diagnóstico del catálogo
              oficial a esta cita. El texto libre de "Notas de la Consulta" no cambia.
            </p>
            {!isCompleted && (
              <div className="cie10-search-wrapper">
                <span className="material-icons-outlined input-icon">search</span>
                <input
                  type="text"
                  placeholder="Ej. J44.9 o &quot;diabetes&quot;"
                  value={cie10SearchTerm}
                  onChange={(e) => { setCie10SearchTerm(e.target.value); setCie10ShowResults(true); }}
                  onFocus={() => setCie10ShowResults(true)}
                  onBlur={() => setTimeout(() => setCie10ShowResults(false), 150)}
                />
                {cie10Searching && <div className="searching-loader">Buscando...</div>}
                {cie10ShowResults && cie10Results.length > 0 && (
                  <ul className="cie10-results">
                    {cie10Results.map((r) => (
                      <li key={r.code} onMouseDown={() => selectCie10Result(r)}>
                        <strong>{r.code}</strong> — {r.name}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )}
            {diagnosisCode ? (
              <div className="cie10-selected-chip">
                <span className="material-icons-outlined">check_circle</span>
                <div>
                  <strong>{diagnosisCode}</strong>
                  {diagnosisCodeLabel && <span> — {diagnosisCodeLabel}</span>}
                </div>
                {!isCompleted && (
                  <button
                    type="button"
                    className="btn-close"
                    onClick={() => { setDiagnosisCode(''); setDiagnosisCodeLabel(''); }}
                  >
                    <span className="material-icons-outlined">close</span>
                  </button>
                )}
              </div>
            ) : (
              <p className="empty-msg">Sin código CIE-10 asociado todavía.</p>
            )}
          </section>

          {/* NOTAS DE LA CONSULTA (EVOLUCIÓN DINÁMICA) */}
          <section className="form-card highlighted-section">
            <h3>
              <span className="material-icons-outlined">analytics</span>
              Notas de la Consulta (Evolución)
            </h3>
            <fieldset disabled={isCompleted} className="form-grid fieldset-plain">
              {sectionsConfig.map((section) => (
                <div className="form-group full-width" key={section.id}>
                  {/* Contenedor flexible para alinear título a la izquierda y checkbox a la derecha */}
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                    <label style={{ margin: 0 }}>
                      {section.label} {section.required && <span className="req-asterisk">*</span>}
                    </label>

                    {/* Checkbox para incluir/excluir este apartado específico de la receta (no aplica en modo lectura) */}
                    {!isCompleted && (
                      <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '13px', color: 'var(--color-primary)', cursor: 'pointer', fontWeight: 500 }}>
                        <input
                          type="checkbox"
                          style={{ width: '16px', height: '16px', accentColor: 'var(--color-primary)', cursor: 'pointer' }}
                          checked={recipeSections[section.label] !== false} // Por defecto true si no está explícitamente en false
                          onChange={(e) => setRecipeSections({
                            ...recipeSections,
                            [section.label]: e.target.checked
                          })}
                        />
                        Incluir en Receta
                      </label>
                    )}
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
            </fieldset>
          </section>

          {isCompleted ? (
            /* BARRA DE ACCIONES EN MODO LECTURA: solo reimprimir la receta ya guardada y volver */
            <div className="recipe-actions-bar">
              <div className="recipe-actions-buttons">
                {appointment?.recipePdfPath && (
                  <a
                    className="btn-secondary"
                    href={toAbsoluteFileUrl(appointment.recipePdfPath)}
                    target="_blank"
                    rel="noreferrer"
                  >
                    <span className="material-icons-outlined">print</span>
                    Ver / Imprimir Receta Guardada
                  </a>
                )}
                <button type="button" className="btn-primary" onClick={() => navigate(goBackTarget)}>
                  <span className="material-icons-outlined">arrow_back</span>
                  Volver
                </button>
              </div>
            </div>
          ) : (
            /* BARRA DE ACCIONES: GENERAR RECETA Y FINALIZAR CONSULTA */
            <div className="recipe-actions-bar">
              <div className="follow-up-field">
                <label htmlFor="follow-up-days">
                  <span className="material-icons-outlined">event_repeat</span>
                  Programar seguimiento (opcional)
                </label>
                <div className="follow-up-input-row">
                  <input
                    id="follow-up-days"
                    type="number"
                    min={1}
                    max={365}
                    placeholder="Días"
                    value={followUpDays}
                    onChange={(e) => setFollowUpDays(e.target.value)}
                  />
                  <span>días después de hoy — aparecerá como recordatorio en el Panel de Control</span>
                </div>
              </div>

              <div className="recipe-actions-buttons">
                {/* PASO 1: GENERA LA RECETA, LA GUARDA EN EL EXPEDIENTE Y ABRE LA IMPRESIÓN */}
                <button
                  type="button"
                  className={`btn-secondary${recipeGenerated ? ' recipe-generated' : ''}`}
                  onClick={handleGenerateAndPrintRecipe}
                  disabled={generatingRecipe || loading || !hasSectionsForRecipe}
                  title={!hasSectionsForRecipe ? 'Marca "Incluir en Receta" en al menos un apartado con contenido para poder generarla' : undefined}
                >
                  <span className="material-icons-outlined">
                    {recipeGenerated ? 'badge' : 'print'}
                  </span>
                  {generatingRecipe ? 'Generando...' : recipeGenerated ? 'Receta Guardada — Reimprimir' : '1. Generar e Imprimir Receta'}
                </button>

                {/* PASO 2: FINALIZAR Y CERRAR EL EXPEDIENTE DE LA CITA */}
                <button
                  type="button"
                  className="btn-primary"
                  onClick={handleFinalize}
                  disabled={loading}
                >
                  <span className="material-icons-outlined">task_alt</span>
                  2. Finalizar Consulta
                </button>
              </div>
              {!hasSectionsForRecipe && !recipeGenerated && (
                <p className="recipe-hint">
                  <span className="material-icons-outlined">info</span>
                  Selecciona al menos un apartado con contenido (casilla "Incluir en Receta") para poder generarla.
                </p>
              )}
            </div>
          )}
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
