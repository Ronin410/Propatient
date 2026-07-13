import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalTime } from '../utils/dateFormatter';
import type { Appointment } from '../types';
import { ConfirmDialog } from '../components/ConfirmDialog';
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

  const [isConfirmModalOpen, setIsConfirmModalOpen] = useState(false);
  const [selectedApp, setSelectedApp] = useState<Appointment | null>(null);

  const fetchDashboardData = async () => {
    setLoading(true);
    try {
      const now = new Date();
      const tzo = -now.getTimezoneOffset();
      const dif = tzo >= 0 ? '+' : '-';
      const pad = (num: number) => num.toString().padStart(2, '0');
      const clientTime = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}T${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}${dif}${pad(Math.floor(Math.abs(tzo) / 60))}:${pad(Math.abs(tzo) % 60)}`;

      const res = await api.get(`/dashboard/summary?clientTime=${encodeURIComponent(clientTime)}`);

      const cleanTodayAppointments = (res.data.todayAppointments || []).filter(
        (app: Appointment) =>
          app.status !== 'COMPLETED' &&
          app.Status !== 'COMPLETED' &&
          app.status !== 'NOSHOW' &&
          app.Status !== 'NOSHOW' &&
          app.status !== 'CANCELLED' &&
          app.Status !== 'CANCELLED'
      );

      setSummary({
        ...res.data,
        todayAppointments: cleanTodayAppointments // Guardamos solo las activas/pendientes
      });
    } catch (err) {
      console.error("Error al cargar datos del dashboard:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDashboardData();
  }, []);

  const handleStartConsultationClick = (app: Appointment) => {
    setSelectedApp(app);
    setIsConfirmModalOpen(true);
  };

  const handleConfirmConsultation = () => {
    if (!selectedApp) return;
    setIsConfirmModalOpen(false);
    navigate(`/consulta/${selectedApp.id || selectedApp.ID}`);
  };

  const [appToCancel, setAppToCancel] = useState<Appointment | null>(null);

  const handleCancelConfirmed = async () => {
    if (!appToCancel) return;
    const id = appToCancel.id || appToCancel.ID;
    try {
      await api.put(`/appointments/${id}/cancel`);
      setAppToCancel(null);
      fetchDashboardData();
    } catch (err) {
      console.error("Error al cancelar la cita:", err);
      setAppToCancel(null);
    }
  };

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '50vh', color: '#005073' }}>
        <div className="spinner-border" role="status"></div>
      </div>
    );
  }

  const nextPatientName = summary?.nextPatient 
    ? `${(summary.nextPatient.patient || summary.nextPatient.Patient)?.firstName || ''} ${(summary.nextPatient.patient || summary.nextPatient.Patient)?.lastName || ''}`
    : 'Sin pacientes en espera';

  return (
    <div className="tracking-page">
      {/* HEADER MODIFICADO CON EL NUEVO BOTÓN */}
      <header className="dashboard-header">
        <div>
          <h1>Panel de Control</h1>
          <p className="subtitle">Seguimiento y flujo de pacientes en tiempo real para el día de hoy.</p>
        </div>
        <button className="btn-submit" onClick={() => navigate('/appointments/new')}>
          <span className="material-icons-outlined">add_task</span>
          Agendar Cita
        </button>
      </header>

      <div className="stats-grid">
        <div className="stat-card">
          <span className="label">Citas del Día</span>
          <span className="value">{summary?.todayCount || 0}</span>
          <span className="desc">Consultas agendadas hoy</span>
        </div>

        <div className="stat-card">
          <span className="label">Pacientes por Atender</span>
          <span className="value">{summary?.pendingCount || 0}</span>
          <span className="desc">En sala de espera / pendientes</span>
        </div>

        <div className="stat-card next-patient-card">
          <span className="label">Siguiente Paciente</span>
          <span className="value">{nextPatientName}</span>
          <span className="desc">
            {summary?.nextPatient ? `Horario: ${formatToLocalTime(summary.nextPatient.appointmentDateTime || summary.nextPatient.appointmentDateTime)}` : 'Línea de espera vacía'}
          </span>
        </div>
      </div>

      <section className="table-section">
        <h2 className="section-title">Lista de Atención del Día</h2>
        
        <div className="table-responsive">
          {summary && summary.todayAppointments && summary.todayAppointments.length > 0 ? (
            <table className="appointments-table">
              <thead>
                <tr>
                  <th>Horario</th>
                  <th>Paciente</th>
                  <th>Motivo de Consulta</th>
                  <th className="action-cell">Acciones</th>
                </tr>
              </thead>
              <tbody>
                {summary.todayAppointments.map((app) => {
                  const patient = app.patient || app.Patient;
                  const pName = `${patient?.firstName || patient?.FirstName || 'Paciente'} ${patient?.lastName || patient?.LastName || ''}`;
                  const appTime = formatToLocalTime(app.appointmentDateTime || app.appointmentDateTime);
                  const isCompleted = app.status === 'completed' || app.Status === 'completed';

                  return (
                    <tr key={app.id || app.ID}>
                      <td className="time-cell">
                        <span className="material-icons-outlined" style={{ fontSize: '18px' }}>schedule</span>
                        {appTime}
                      </td>
                      <td className="patient-name">{pName}</td>
                      <td className="reason-cell" title={app.reason || app.Reason}>
                        {app.reason || app.Reason || 'Consulta General'}
                      </td>
                      <td className="action-cell">
                        {isCompleted ? (
                          <span style={{ fontSize: '13px', color: '#137333', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                            ✓ Atendido
                          </span>
                        ) : (
                          <div style={{ display: 'flex', gap: '8px' }}>
                            <button className="btn-primary" onClick={() => handleStartConsultationClick(app)}>
                              <span className="material-icons-outlined" style={{ fontSize: '16px' }}>play_arrow</span>
                              Iniciar Atencion
                            </button>
                            <button className="btn-text" onClick={() => setAppToCancel(app)}>
                              Cancelar
                            </button>
                          </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          ) : (
            <p className="empty-msg">No hay citas programadas para el día de hoy.</p>
          )}
        </div>
      </section>

      {isConfirmModalOpen && selectedApp && (
        <div className="modal-overlay">
          <div className="modal-content">
            <div className="modal-header">
              <span className="material-icons-outlined alert-icon">start</span>
              <h3>¿Deseas iniciar la consulta?</h3>
            </div>
            <p>
              Estás por iniciar la consulta médica y registrar la evolución para: <br />
              <strong>
                {(selectedApp.patient || selectedApp.Patient)?.firstName || ''} {(selectedApp.patient || selectedApp.Patient)?.lastName || ''}
              </strong>
            </p>
            <div className="modal-footer">
              <button className="btn-text" onClick={() => setIsConfirmModalOpen(false)}>Regresar</button>
              <button className="btn-primary" onClick={handleConfirmConsultation}>Confirmar e Iniciar</button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog
        isOpen={!!appToCancel}
        variant="danger"
        title="Cancelar cita"
        message={`¿Seguro que quieres cancelar la cita de ${(appToCancel?.patient || appToCancel?.Patient)?.firstName || 'este paciente'}? Esta acción no se puede deshacer.`}
        confirmText="Sí, cancelar"
        cancelText="Regresar"
        onConfirm={handleCancelConfirmed}
        onCancel={() => setAppToCancel(null)}
      />
    </div>
  );
};