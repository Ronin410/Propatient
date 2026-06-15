import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalDate, formatToLocalTime } from '../utils/dateFormatter';
import type { Patient, MedicalHistory } from '../types';
import './PatientDetail.scss';

export const PatientDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [patient, setPatient] = useState<Patient | null>(null);
  const [isEditingHistory, setIsEditingHistory] = useState(false);
  const [editedHistory, setEditedHistory] = useState<MedicalHistory | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  const handleSaveHistory = async () => {
    try {
      // Endpoint: PUT /api/patients/:id/medical-history
      await api.put(`/patients/${id}/medical-history`, editedHistory);
      setIsEditingHistory(false);
      fetchPatientData(); // Recargar datos
    } catch (error) {
      alert("Error al guardar el historial");
    }
  };

  const fetchPatientData = async () => {
    setLoading(true);
    try {
      // Endpoint: /api/patients/:id/history (definido en Go con Preloads)
      const response = await api.get(`/patients/${id}/history`);
      if (response.data) {
        setPatient(response.data);
        setEditedHistory(response.data.MedicalHistory || {});
      }
    } catch (error) {
      console.error("Error al cargar detalle del paciente:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (id) fetchPatientData();
  }, [id]);

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
        </div>
        <div className="header-actions">
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
            <p><strong>Teléfono:</strong> {patient.phone || 'N/A'}</p>
            <p><strong>Email:</strong> {patient.email}</p>
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
                
                <label>Patológicos</label>
                <textarea value={editedHistory?.pathological_history} onChange={e => setEditedHistory({...editedHistory!, pathological_history: e.target.value})} />
                
                <label>No Patológicos</label>
                <textarea value={editedHistory?.non_pathological_history} onChange={e => setEditedHistory({...editedHistory!, non_pathological_history: e.target.value})} />
              </div>
            ) : (
              <>
                <div className="history-item">
                  <label>Alergias:</label>
                  <p>{patient.MedicalHistory?.allergies || 'Ninguna registrada'}</p>
                </div>
                <div className="history-item">
                  <label>Patológicos:</label>
                  <p>{patient.MedicalHistory?.pathological_history || 'Sin registros'}</p>
                </div>
                <div className="history-item">
                  <label>No Patológicos:</label>
                  <p>{patient.MedicalHistory?.non_pathological_history || 'Sin registros'}</p>
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
              {patient.Appointments && patient.Appointments.length > 0 ? (
                patient.Appointments.map((app) => (
                  <div key={app.id} className="timeline-item">
                    <div className="item-date">
                      <span>{app.appointmentDateTime ? formatToLocalDate(app.appointmentDateTime) : 'N/A'}</span>
                      <small>{app.appointmentDateTime ? formatToLocalTime(app.appointmentDateTime) : 'N/A'}</small>
                    </div>
                    <div className="item-content">
                      <span className={`status-tag ${app.status.toLowerCase()}`}>{app.status}</span>
                      <h4>{app.reason}</h4>
                      <p>{app.observations || 'Sin notas'}</p>
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