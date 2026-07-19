import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalDate, formatToLocalTime } from '../utils/dateFormatter';
import type { Patient, MedicalHistory } from '../types';
import { useFetchData } from '../hooks/useFetchData';
import { downloadPatientHistoryPDF } from '../utils/patientHistoryPdf';
import { toAbsoluteFileUrl } from '../utils/fileUrl';
import './PatientDetail.scss';

export const PatientDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [isEditingHistory, setIsEditingHistory] = useState(false);
  const [editedHistory, setEditedHistory] = useState<MedicalHistory | null>(null);

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
                <textarea value={editedHistory?.allergies} onChange={e => setEditedHistory({...editedHistory!, allergies: e.target.value})} />

                <label>Medicamentos Actuales</label>
                <textarea value={editedHistory?.current_medication} onChange={e => setEditedHistory({...editedHistory!, current_medication: e.target.value})} />

                <label>Antecedentes Patológicos</label>
                <textarea value={editedHistory?.pathological_history} onChange={e => setEditedHistory({...editedHistory!, pathological_history: e.target.value})} />

                <label>Antecedentes Quirúrgicos y Traumas</label>
                <textarea value={editedHistory?.surgical_history} onChange={e => setEditedHistory({...editedHistory!, surgical_history: e.target.value})} />

                <label>No Patológicos</label>
                <textarea value={editedHistory?.non_pathological_history} onChange={e => setEditedHistory({...editedHistory!, non_pathological_history: e.target.value})} />

                <label>Heredofamiliares</label>
                <textarea value={editedHistory?.hereditaryHistory} onChange={e => setEditedHistory({...editedHistory!, hereditaryHistory: e.target.value})} />

                <label>Hábitos y Estilo de Vida</label>
                <textarea value={editedHistory?.habitsLifestyle} onChange={e => setEditedHistory({...editedHistory!, habitsLifestyle: e.target.value})} />

                <label>Ginecoobstétricos</label>
                <textarea value={editedHistory?.gynecoObstetric} onChange={e => setEditedHistory({...editedHistory!, gynecoObstetric: e.target.value})} />
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
        </div>
      </div>
    </div>
  );
};