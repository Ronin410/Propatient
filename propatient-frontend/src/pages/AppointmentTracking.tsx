import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalTime } from '../utils/dateFormatter';
import type { Appointment } from '../types';
import './AppointmentTracking.scss';

interface DashboardSummary {
  todayCount: number;
  pendingCount: number;
  todayAppointments: Appointment[];
  nextPatient: Appointment | null;
}

export const AppointmentTracking: React.FC = () => {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  // Estados para el modal de confirmación
  const [isConfirmModalOpen, setIsConfirmModalOpen] = useState(false);
  const [selectedApp, setSelectedApp] = useState<Appointment | null>(null);

  const fetchDashboardData = async () => {
    setLoading(true);
    try {
      // Generar una cadena ISO que incluya el offset local (ej: 2026-04-12T22:04:12-06:00)
      const now = new Date();
      const tzo = -now.getTimezoneOffset();
      const dif = tzo >= 0 ? '+' : '-';
      const pad = (num: number) => num.toString().padStart(2, '0');
      const clientTime = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}T${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}${dif}${pad(Math.floor(Math.abs(tzo) / 60))}:${pad(Math.abs(tzo) % 60)}`;

      const response = await api.get(`/dashboard/summary?clientTime=${clientTime}`);
      setSummary(response.data);
    } catch (error) {
      console.error("Error cargando dashboard:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDashboardData();
  }, []);

  const handleOpenModal = (app: Appointment) => {
    setSelectedApp(app);
    setIsConfirmModalOpen(true);
  };

  const handleConfirmConsultation = () => {
    if (selectedApp) navigate(`/consulta/${selectedApp.id}`);
    setIsConfirmModalOpen(false);
  };

  if (loading) return <div className="loading-state">Cargando resumen del día...</div>;

  const todayApps = summary?.todayAppointments || [];

  return (
    <div className="tracking-page">
      <header className="dashboard-header">
        <h1>Panel de Control</h1>
        <p className="subtitle">Bienvenido de nuevo, Dr.</p>
      </header>

      <div className="stats-grid">
        <div className="card stat-card">
          <span className="label">Citas de Hoy</span>
          <span className="value">{summary?.todayCount || 0}</span>
        </div>
        <div className="card stat-card highlight">
          <span className="label">Total Pendientes</span>
          <span className="value">{summary?.pendingCount || 0}</span>
        </div>
        {summary?.nextPatient && (
          <div className="card stat-card next-patient-card">
            <span className="label">Siguiente Paciente</span>
            <span className="patient-name">
              {(summary.nextPatient?.patient || summary.nextPatient?.Patient)?.firstName || 
               (summary.nextPatient?.patient || summary.nextPatient?.Patient)?.FirstName || 'Paciente'} 
              {' '}
              {(summary.nextPatient?.patient || summary.nextPatient?.Patient)?.lastName || 
               (summary.nextPatient?.patient || summary.nextPatient?.Patient)?.LastName || ''}
            </span>
            <span className="time">{summary.nextPatient.appointmentDateTime ? formatToLocalTime(summary.nextPatient.appointmentDateTime) : 'N/A'}</span>
            <button className="btn-primary-sm" onClick={() => handleOpenModal(summary.nextPatient!)}>
              Atender Ahora
            </button>
          </div>
        )}
      </div>

      <section className="agenda-section">
        <h2>Agenda del Día</h2>
        <div className="card table-container">
          {todayApps.length > 0 ? (
            <div className="table-responsive">
              <table className="dashboard-table completed-table">
                <thead>
                  <tr>
                    <th>Hora</th>
                    <th>Paciente</th>
                    <th>Motivo</th>
                    <th>Estado</th>
                    <th>Acción</th>
                  </tr>
                </thead>
                <tbody>
                  {todayApps.map((app) => {
                    const p = app.patient || app.Patient;
                    return (
                      <tr key={app.id}>
                        <td className="time-cell">{app.appointmentDateTime ? formatToLocalTime(app.appointmentDateTime) : 'N/A'}</td>
                        <td className="patient-cell">
                          {p?.firstName || p?.FirstName || 'N/A'} {p?.lastName || p?.LastName || ''}
                        </td>
                        <td className="reason-cell">{app.reason}</td>
                        <td>
                          <span className={`status-badge ${app.status.toLowerCase()}`}>
                            {app.status.toUpperCase() === 'IN_COURSE' ? 'En Curso' : 
                             app.status.toUpperCase() === 'COMPLETED' ? 'Finalizada' : 'Pendiente'}
                          </span>
                        </td>
                        <td className="action-cell">
                          {app.status.toUpperCase() !== 'COMPLETED' && (
                            <button 
                              className="btn-primary-sm" 
                              style={{ marginRight: '8px' }} 
                              onClick={() => handleOpenModal(app)}
                            >
                              Atender
                            </button>
                          )}
                          {(p?.id || (p as any)?.ID) && ( 
                            <button className="btn-text" onClick={() => navigate(`/pacientes/${p?.id || (p as any)?.ID}`)}>Ver Ficha</button>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="empty-msg">No hay citas programadas para hoy.</p>
          )}
        </div>
      </section>

      {/* Modal de Confirmación de Inicio de Consulta */}
      {isConfirmModalOpen && selectedApp && (
        <div className="modal-overlay">
          <div className="modal-content card">
            <div className="modal-header">
              <span className="material-icons-outlined alert-icon">medical_services</span>
              <h3>¿Iniciar Consulta Médica?</h3>
            </div>
            <p>
              Estás a punto de iniciar la atención para: <br />
              <strong>
                {(selectedApp.patient || selectedApp.Patient)?.firstName || (selectedApp.patient || selectedApp.Patient)?.FirstName || 'Paciente'} 
                {' '}
                {(selectedApp.patient || selectedApp.Patient)?.lastName || (selectedApp.patient || selectedApp.Patient)?.LastName || ''}
              </strong>
            </p>
            <div className="modal-footer">
              <button className="btn-text" onClick={() => setIsConfirmModalOpen(false)}>Cancelar</button>
              <button className="btn-primary" onClick={handleConfirmConsultation}>Confirmar e Iniciar</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};