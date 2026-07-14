import React, { useEffect, useRef, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import './ConsultationManager.scss';
import { useConsultation, type AppointmentFile } from '../hooks/useConsultation';
import { toAbsoluteFileUrl } from '../utils/fileUrl';

type FormSection = 'generalData' | 'medicalHistory';

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

  const toggleQR = () => {
    if (!showQR) {
      const uploadUrl = `${window.location.origin}/public-upload/${appointmentId}`;
      setQrImageUrl(`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(uploadUrl)}`);
    }
    setShowQR(!showQR);
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
          <button className="btn-outline-danger" onClick={() => navigate(goBackTarget)}>
            {isCompleted ? 'Volver' : 'Pausar / Salir'}
          </button>
        </div>
      </header>

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
                </fieldset>
              ) : (
                /* PANEL DE ANTECEDENTES REESTRUCTURADO */
                <fieldset disabled={isCompleted} className="medical-history-sections fieldset-plain">
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
                          rows={3}
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
                          rows={3}
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
                          rows={3}
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
                          rows={3}
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
                <img src={qrImageUrl} alt="QR de carga" />
                <p className="qr-desc">Pida al paciente que escanee este código para subir fotos desde su móvil.</p>
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
