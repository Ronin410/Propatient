import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalDate, formatToLocalTime } from '../utils/dateFormatter';
import type { Patient, MedicalHistory } from '../types';
import { useFetchData } from '../hooks/useFetchData';
import { downloadPatientHistoryPDF } from '../utils/patientHistoryPdf';
import { toAbsoluteFileUrl } from '../utils/fileUrl';
import {
  ChecklistField, HabitsLifestyleField, GynecoObstetricField,
  ALLERGY_OPTIONS, PATHOLOGICAL_OPTIONS, SURGICAL_OPTIONS, HEREDITARY_OPTIONS,
} from '../components/MedicalHistoryFields';
import './PatientDetail.scss';

interface AuditLogEntry {
  id: number;
  createdAt: string;
  actorRole: 'MEDICO' | 'STAFF';
  actorName: string;
  action: 'created' | 'updated' | 'viewed' | 'deleted';
  entityType: string;
  details: string;
  ipAddress: string;
}

const auditActionLabels: Record<AuditLogEntry['action'], string> = {
  created: 'Creación',
  updated: 'Modificación',
  viewed: 'Consulta',
  deleted: 'Eliminación',
};

export const PatientDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [isEditingHistory, setIsEditingHistory] = useState(false);
  const [editedHistory, setEditedHistory] = useState<MedicalHistory | null>(null);

  const [showAuditLog, setShowAuditLog] = useState(false);
  const [auditEntries, setAuditEntries] = useState<AuditLogEntry[] | null>(null);
  const [auditLoading, setAuditLoading] = useState(false);

  const handleToggleAuditLog = async () => {
    const next = !showAuditLog;
    setShowAuditLog(next);
    if (next && auditEntries === null) {
      setAuditLoading(true);
      try {
        const res = await api.get(`/patients/${id}/audit-log`);
        setAuditEntries(res.data || []);
      } catch {
        setAuditEntries([]);
      } finally {
        setAuditLoading(false);
      }
    }
  };

  // Endpoint: /api/patients/:id/history (definido en Go con Preloads)
  const { data: patient, loading, refetch } = useFetchData<Patient>(
    () => api.get(`/patients/${id}/history`).then((res) => res.data),
    [id]
  );

  useEffect(() => {
    if (patient) setEditedHistory(patient.medicalHistory || null);
  }, [patient]);

  const handleSaveHistory = async () => {
    try {
      // Endpoint: PUT /api/patients/:id/medical-history
      await api.put(`/patients/${id}/medical-history`, editedHistory);
      setIsEditingHistory(false);
      refetch();
    } catch (error) {
      alert("Error al guardar el historial");
    }
  };

  if (loading) return <div className="loading-state">Cargando información del paciente...</div>;
  if (!patient) return <div className="error-state">Paciente no encontrado.</div>;

  return (
    <div className="patient-detail-container">
      <header className="page-header">
        <div className="header-info">
          <button className="btn-back" onClick={() => navigate('/pacientes')}>
            <span className="material-icons-outlined">arrow_back</span>
          </button>
          <h1>{patient.firstName} {patient.lastName}</h1>
          {patient.isMinor && <span className="minor-badge">Menor de edad</span>}
        </div>
        <div className="header-actions">
          <button className="btn-outline-sm" onClick={() => downloadPatientHistoryPDF(patient)}>
            Exportar Historial (PDF)
          </button>
          <button className="btn-outline-sm" onClick={() => navigate(`/pacientes/editar/${id}`)}>
            Editar Perfil
          </button>
          <button className="btn-primary" onClick={() => navigate(`/appointments/new?patientId=${id}`)}>
            Agendar Cita
          </button>
        </div>
      </header>

      <div className="detail-grid">
        {/* Columna Izquierda: Información y Antecedentes */}
        <div className="main-info">
          <section className="card info-card">
            <h3>Información General</h3>
            <p><strong>Teléfono{patient.isMinor ? ' (padre/madre o tutor)' : ''}:</strong> {patient.phone || 'N/A'}</p>
            <p><strong>Email{patient.isMinor ? ' (padre/madre o tutor)' : ''}:</strong> {patient.email}</p>
            <p><strong>F. Nacimiento:</strong> {patient.birthDate ? formatToLocalDate(patient.birthDate) : 'N/A'}</p>
          </section>

          <section className="card history-card">
            <div className="card-header-flex">
              <h3>Antecedentes Médicos</h3>
              {!isEditingHistory ? (
                <button className="btn-text" onClick={() => setIsEditingHistory(true)}>Editar</button>
              ) : (
                <div className="actions">
                  <button className="btn-text-danger" onClick={() => setIsEditingHistory(false)}>Cancelar</button>
                  <button className="btn-text-success" onClick={handleSaveHistory}>Guardar</button>
                </div>
              )}
            </div>
            
            {isEditingHistory ? (
              <div className="history-edit-form">
                <label>Alergias</label>
                <ChecklistField
                  key={`allergies-${patient.id}`}
                  value={editedHistory?.allergies || ''}
                  options={ALLERGY_OPTIONS}
                  noneLabel="Sin alergias conocidas"
                  otherPlaceholder="Otra alergia u observación..."
                  onChange={next => setEditedHistory({...editedHistory!, allergies: next})}
                />

                <label>Medicamentos Actuales</label>
                <ChecklistField
                  key={`medication-${patient.id}`}
                  value={editedHistory?.current_medication || ''}
                  noneLabel="No toma medicamentos actualmente"
                  otherLabel="Medicamento, dosis y frecuencia"
                  otherPlaceholder="Ej: Metformina 850mg c/24h..."
                  onChange={next => setEditedHistory({...editedHistory!, current_medication: next})}
                />

                <label>Antecedentes Patológicos</label>
                <ChecklistField
                  key={`pathological-${patient.id}`}
                  value={editedHistory?.pathological_history || ''}
                  options={PATHOLOGICAL_OPTIONS}
                  noneLabel="Sin antecedentes patológicos"
                  otherPlaceholder="Otra enfermedad crónica u observación..."
                  onChange={next => setEditedHistory({...editedHistory!, pathological_history: next})}
                />

                <label>Antecedentes Quirúrgicos y Traumas</label>
                <ChecklistField
                  key={`surgical-${patient.id}`}
                  value={editedHistory?.surgical_history || ''}
                  options={SURGICAL_OPTIONS}
                  noneLabel="Sin cirugías ni traumatismos previos"
                  otherPlaceholder="Otra cirugía, hospitalización o fractura..."
                  onChange={next => setEditedHistory({...editedHistory!, surgical_history: next})}
                />

                <label>No Patológicos</label>
                <textarea value={editedHistory?.non_pathological_history} onChange={e => setEditedHistory({...editedHistory!, non_pathological_history: e.target.value})} />

                <label>Heredofamiliares</label>
                <ChecklistField
                  key={`hereditary-${patient.id}`}
                  value={editedHistory?.hereditaryHistory || ''}
                  options={HEREDITARY_OPTIONS}
                  noneLabel="Sin antecedentes heredofamiliares relevantes"
                  otherLabel="Quién / detalles"
                  otherPlaceholder="Ej: Madre - diabetes, Abuelo paterno - cáncer..."
                  onChange={next => setEditedHistory({...editedHistory!, hereditaryHistory: next})}
                />

                <label>Hábitos y Estilo de Vida</label>
                <HabitsLifestyleField
                  key={`habits-${patient.id}`}
                  value={editedHistory?.habitsLifestyle || ''}
                  onChange={next => setEditedHistory({...editedHistory!, habitsLifestyle: next})}
                />

                <label>Ginecoobstétricos</label>
                <GynecoObstetricField
                  key={`gyneco-${patient.id}`}
                  value={editedHistory?.gynecoObstetric || ''}
                  onChange={next => setEditedHistory({...editedHistory!, gynecoObstetric: next})}
                />
              </div>
            ) : (
              <>
                <div className="history-item">
                  <label>Alergias:</label>
                  <p>{patient.medicalHistory?.allergies || 'Ninguna registrada'}</p>
                </div>
                <div className="history-item">
                  <label>Medicamentos Actuales:</label>
                  <p>{patient.medicalHistory?.current_medication || 'Sin registros'}</p>
                </div>
                <div className="history-item">
                  <label>Antecedentes Patológicos:</label>
                  <p>{patient.medicalHistory?.pathological_history || 'Sin registros'}</p>
                </div>
                <div className="history-item">
                  <label>Antecedentes Quirúrgicos y Traumas:</label>
                  <p>{patient.medicalHistory?.surgical_history || 'Sin registros'}</p>
                </div>
                <div className="history-item">
                  <label>No Patológicos:</label>
                  <p>{patient.medicalHistory?.non_pathological_history || 'Sin registros'}</p>
                </div>
                <div className="history-item">
                  <label>Heredofamiliares:</label>
                  <p>{patient.medicalHistory?.hereditaryHistory || 'Sin registros'}</p>
                </div>
                <div className="history-item">
                  <label>Hábitos y Estilo de Vida:</label>
                  <p>{patient.medicalHistory?.habitsLifestyle || 'Sin registros'}</p>
                </div>
                <div className="history-item">
                  <label>Ginecoobstétricos:</label>
                  <p>{patient.medicalHistory?.gynecoObstetric || 'Sin registros'}</p>
                </div>
              </>
            )}
          </section>
        </div>

        {/* Columna Derecha: Cronología de Consultas */}
        <div className="appointments-timeline">
          <section className="card timeline-card">
            <h3>Historial de Consultas</h3>
            <div className="timeline">
              {patient.appointments && patient.appointments.length > 0 ? (
                patient.appointments.map((app) => (
                  <div key={app.id} className="timeline-item">
                    <div className="item-date">
                      <span>{app.appointmentDateTime ? formatToLocalDate(app.appointmentDateTime) : 'N/A'}</span>
                      <small>{app.appointmentDateTime ? formatToLocalTime(app.appointmentDateTime) : 'N/A'}</small>
                    </div>
                    <div className="item-content">
                      <span className={`status-tag ${app.status.toLowerCase()}`}>{app.status}</span>
                      <h4>{app.reason}</h4>
                      <p>{app.observations || 'Sin notas'}</p>
                      {app.status === 'COMPLETED' && (
                        <div className="timeline-item-actions">
                          <button className="btn-text" onClick={() => navigate(`/consulta/${app.id}`)}>
                            Ver Consulta
                          </button>
                          {app.recipePdfPath && (
                            <a
                              className="btn-text"
                              href={toAbsoluteFileUrl(app.recipePdfPath)}
                              target="_blank"
                              rel="noreferrer"
                            >
                              Imprimir Receta
                            </a>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                ))
              ) : (
                <p className="empty-msg">No hay citas previas registradas.</p>
              )}
            </div>
          </section>

          <section className="card audit-log-card">
            <div className="card-header-flex">
              <h3>Bitácora de auditoría</h3>
              <button className="btn-text" onClick={handleToggleAuditLog}>
                {showAuditLog ? 'Ocultar' : 'Ver bitácora'}
              </button>
            </div>
            <p className="audit-log-subtitle">
              Quién accedió o modificó este expediente, cuándo y desde qué dirección — registro
              exigido por la normativa de expedientes clínicos electrónicos (NOM-024).
            </p>
            {showAuditLog && (
              auditLoading ? (
                <p className="empty-msg">Cargando...</p>
              ) : !auditEntries || auditEntries.length === 0 ? (
                <p className="empty-msg">Sin eventos registrados todavía.</p>
              ) : (
                <ul className="audit-log-list">
                  {auditEntries.map((entry) => (
                    <li key={entry.id} className={`audit-log-item action-${entry.action}`}>
                      <div className="audit-log-when">
                        <span>{formatToLocalDate(entry.createdAt)}</span>
                        <small>{formatToLocalTime(entry.createdAt)}</small>
                      </div>
                      <div className="audit-log-what">
                        <strong>{auditActionLabels[entry.action]}</strong>
                        <span> — {entry.details}</span>
                        <div className="audit-log-actor">
                          {entry.actorName || 'Cuenta eliminada'} ({entry.actorRole === 'STAFF' ? 'personal' : 'doctor'}) · IP {entry.ipAddress || 'desconocida'}
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>
              )
            )}
          </section>
        </div>
      </div>
    </div>
  );
};