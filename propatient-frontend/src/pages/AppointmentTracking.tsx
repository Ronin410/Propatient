import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalTime, formatToLocalDate } from '../utils/dateFormatter';
import type { Appointment } from '../types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { useFetchData } from '../hooks/useFetchData';
import { useAuth } from '../context/AuthContext';
import { getErrorMessage } from '../utils/errorMessage';
import './AppointmentTracking.scss';

interface DashboardSummary {
  todayCount: number;
  pendingCount: number;
  todayAppointments: Appointment[];
  nextPatient: Appointment | null;
}

interface ConsultorioStats {
  totalPatients: number;
  appointmentsThisMonth: number;
  completedThisMonth: number;
  cancelledThisMonth: number;
  noShowThisMonth: number;
  noShowRate: number;
  upcomingAppointments: number;
}

export const AppointmentTracking: React.FC = () => {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();
  const { isStaff } = useAuth();

  const [isConfirmModalOpen, setIsConfirmModalOpen] = useState(false);
  const [selectedApp, setSelectedApp] = useState<Appointment | null>(null);

  const { data: consultorioStats } = useFetchData<ConsultorioStats>(
    () => api.get('/dashboard/stats').then((res) => res.data)
  );

  const { data: followUps, setData: setFollowUps } = useFetchData<Appointment[]>(
    () => api.get('/dashboard/follow-ups').then((res) => res.data)
  );

  // Solicitudes de agendamiento público (sin cuenta) que aún no confirma el
  // consultorio: ver ConfirmAppointment/CreatePublicAppointment en el backend.
  const { data: pendingRequests, setData: setPendingRequests } = useFetchData<Appointment[]>(
    () => api.get('/appointments?status=PENDING_CONFIRMATION').then((res) => res.data)
  );
  const [requestActionId, setRequestActionId] = useState<number | null>(null);

  const handleConfirmRequest = async (appointmentId: number) => {
    setRequestActionId(appointmentId);
    try {
      await api.put(`/appointments/${appointmentId}/confirm`);
      setPendingRequests((prev) => (prev || []).filter((a) => a.id !== appointmentId));
      fetchDashboardData();
    } catch (err) {
      console.error('Error al confirmar la solicitud de cita:', err);
    } finally {
      setRequestActionId(null);
    }
  };

  const handleRejectRequest = async (appointmentId: number) => {
    setRequestActionId(appointmentId);
    try {
      await api.put(`/appointments/${appointmentId}/cancel`);
      setPendingRequests((prev) => (prev || []).filter((a) => a.id !== appointmentId));
    } catch (err) {
      console.error('Error al rechazar la solicitud de cita:', err);
    } finally {
      setRequestActionId(null);
    }
  };

  const handleDismissFollowUp = async (appointmentId: number) => {
    try {
      await api.put(`/appointments/${appointmentId}`, { followUpDate: null });
      setFollowUps((prev) => (prev || []).filter((a) => a.id !== appointmentId));
    } catch (err) {
      console.error('Error al descartar el seguimiento:', err);
    }
  };

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
          app.status !== 'NOSHOW' &&
          app.status !== 'CANCELLED' &&
          app.status !== 'PENDING_CONFIRMATION'
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
    navigate(`/consulta/${selectedApp.id}`);
  };

  const [appToCancel, setAppToCancel] = useState<Appointment | null>(null);

  const handleCancelConfirmed = async () => {
    if (!appToCancel) return;
    const id = appToCancel.id;
    try {
      await api.put(`/appointments/${id}/cancel`);
      setAppToCancel(null);
      fetchDashboardData();
    } catch (err) {
      console.error("Error al cancelar la cita:", err);
      setAppToCancel(null);
    }
  };

  // --- Reprogramar cita ---
  const [appToReschedule, setAppToReschedule] = useState<Appointment | null>(null);
  const [newDateTime, setNewDateTime] = useState('');
  const [rescheduling, setRescheduling] = useState(false);
  const [rescheduleError, setRescheduleError] = useState<string | null>(null);

  // Convierte un ISO string a formato "YYYY-MM-DDTHH:mm" en hora local,
  // como lo espera un <input type="datetime-local">.
  const toDatetimeLocalValue = (iso: string) => {
    const d = new Date(iso);
    const pad = (n: number) => n.toString().padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  };

  const handleRescheduleClick = (app: Appointment) => {
    setAppToReschedule(app);
    setNewDateTime(toDatetimeLocalValue(app.appointmentDateTime));
    setRescheduleError(null);
  };

  const handleConfirmReschedule = async () => {
    if (!appToReschedule || !newDateTime) return;
    setRescheduling(true);
    setRescheduleError(null);
    try {
      await api.put(`/appointments/${appToReschedule.id}`, {
        appointmentDateTime: new Date(newDateTime).toISOString(),
      });
      setAppToReschedule(null);
      fetchDashboardData();
    } catch (err) {
      console.error("Error al reprogramar la cita:", err);
      setRescheduleError(getErrorMessage(err, 'No se pudo reprogramar la cita. Intenta de nuevo.'));
    } finally {
      setRescheduling(false);
    }
  };

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '50vh', color: 'var(--color-primary)' }}>
        <div className="spinner-border" role="status"></div>
      </div>
    );
  }

  const nextPatientName = summary?.nextPatient
    ? `${summary.nextPatient.patient?.firstName || ''} ${summary.nextPatient.patient?.lastName || ''}`
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
            {summary?.nextPatient ? `Horario: ${formatToLocalTime(summary.nextPatient.appointmentDateTime)}` : 'Línea de espera vacía'}
          </span>
        </div>
      </div>

      {/* PANEL DE MÉTRICAS DEL CONSULTORIO */}
      <section className="table-section">
        <h2 className="section-title">Métricas del Consultorio</h2>
        <div className="stats-grid">
          <div className="stat-card">
            <span className="label">Total de Pacientes</span>
            <span className="value">{consultorioStats?.totalPatients ?? '—'}</span>
            <span className="desc">Pacientes vinculados a tu consulta</span>
          </div>

          <div className="stat-card">
            <span className="label">Citas Este Mes</span>
            <span className="value">{consultorioStats?.appointmentsThisMonth ?? '—'}</span>
            <span className="desc">
              {consultorioStats
                ? `${consultorioStats.completedThisMonth} completadas · ${consultorioStats.cancelledThisMonth} canceladas`
                : 'Cargando...'}
            </span>
          </div>

          <div className="stat-card">
            <span className="label">Tasa de Inasistencia</span>
            <span className="value">{consultorioStats ? `${consultorioStats.noShowRate.toFixed(1)}%` : '—'}</span>
            <span className="desc">Histórico de citas no atendidas</span>
          </div>

          <div className="stat-card">
            <span className="label">Próximas Citas</span>
            <span className="value">{consultorioStats?.upcomingAppointments ?? '—'}</span>
            <span className="desc">Pendientes en los próximos 30 días</span>
          </div>
        </div>
      </section>

      {/* SOLICITUDES DE CITA PÚBLICAS PENDIENTES DE CONFIRMAR */}
      {pendingRequests && pendingRequests.length > 0 && (
        <section className="table-section">
          <h2 className="section-title">Solicitudes de Cita Nuevas</h2>
          <div className="follow-up-list">
            {pendingRequests.map((app) => (
              <div key={app.id} className="follow-up-item">
                <div className="follow-up-info">
                  <span className="material-icons-outlined">mark_email_unread</span>
                  <div>
                    <strong>{app.patient?.firstName || 'Paciente'} {app.patient?.lastName || ''}</strong>
                    <span className="follow-up-date">
                      Solicita cita para el {formatToLocalDate(app.appointmentDateTime)} a las {formatToLocalTime(app.appointmentDateTime)}
                      {app.reason ? ` · ${app.reason}` : ''}
                    </span>
                  </div>
                </div>
                <div className="follow-up-actions">
                  <button
                    className="btn-primary"
                    disabled={requestActionId === app.id}
                    onClick={() => handleConfirmRequest(app.id)}
                  >
                    Confirmar
                  </button>
                  <button
                    className="btn-text"
                    disabled={requestActionId === app.id}
                    onClick={() => handleRejectRequest(app.id)}
                  >
                    Rechazar
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* SEGUIMIENTO PROGRAMADO */}
      {followUps && followUps.length > 0 && (
        <section className="table-section">
          <h2 className="section-title">Seguimiento Pendiente</h2>
          <div className="follow-up-list">
            {followUps.map((app) => {
              const isOverdue = new Date(app.followUpDate!) < new Date();
              return (
                <div key={app.id} className={`follow-up-item ${isOverdue ? 'overdue' : ''}`}>
                  <div className="follow-up-info">
                    <span className="material-icons-outlined">event_repeat</span>
                    <div>
                      <strong>{app.patient?.firstName || 'Paciente'} {app.patient?.lastName || ''}</strong>
                      <span className="follow-up-date">
                        {isOverdue ? 'Seguimiento vencido: ' : 'Seguimiento sugerido: '}
                        {formatToLocalDate(app.followUpDate!)}
                      </span>
                    </div>
                  </div>
                  <div className="follow-up-actions">
                    <button className="btn-primary" onClick={() => navigate(`/appointments/new?patientId=${app.patient?.id}`)}>
                      Agendar Cita
                    </button>
                    <button className="btn-text" onClick={() => handleDismissFollowUp(app.id)}>
                      Descartar
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      )}

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
                  <th>Teléfono</th>
                  <th className="action-cell">Acciones</th>
                </tr>
              </thead>
              <tbody>
                {summary.todayAppointments.map((app) => {
                  const patient = app.patient;
                  const pName = `${patient?.firstName || 'Paciente'} ${patient?.lastName || ''}`;
                  const appTime = formatToLocalTime(app.appointmentDateTime);
                  const isCompleted = app.status === 'COMPLETED';

                  return (
                    <tr key={app.id}>
                      <td className="time-cell">
                        <span className="material-icons-outlined" style={{ fontSize: '18px' }}>schedule</span>
                        {appTime}
                      </td>
                      <td className="patient-name">{pName}</td>
                      <td className="reason-cell" title={app.reason}>
                        {app.reason || 'Consulta General'}
                      </td>
                      <td className="phone-cell">{patient?.phone || '—'}</td>
                      <td className="action-cell">
                        {isCompleted ? (
                          <span style={{ fontSize: '13px', color: 'var(--color-success)', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: '4px' }}>
                            ✓ Atendido
                          </span>
                        ) : (
                          <div style={{ display: 'inline-flex', gap: '8px' }}>
                            {!isStaff && (
                              <button className="btn-primary" onClick={() => handleStartConsultationClick(app)}>
                                <span className="material-icons-outlined" style={{ fontSize: '16px' }}>play_arrow</span>
                                Iniciar Atencion
                              </button>
                            )}
                            <button className="btn-text" onClick={() => handleRescheduleClick(app)}>
                              Reprogramar
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
                {selectedApp.patient?.firstName || ''} {selectedApp.patient?.lastName || ''}
              </strong>
            </p>
            <div className="modal-footer">
              <button className="btn-text" onClick={() => setIsConfirmModalOpen(false)}>Regresar</button>
              <button className="btn-primary" onClick={handleConfirmConsultation}>Confirmar e Iniciar</button>
            </div>
          </div>
        </div>
      )}

      {appToReschedule && (
        <div className="modal-overlay">
          <div className="modal-content">
            <div className="modal-header">
              <span className="material-icons-outlined alert-icon">event_repeat</span>
              <h3>Reprogramar cita</h3>
            </div>
            <p>
              Nueva fecha y hora para la cita de: <br />
              <strong>
                {appToReschedule.patient?.firstName || ''} {appToReschedule.patient?.lastName || ''}
              </strong>
            </p>
            <input
              type="datetime-local"
              value={newDateTime}
              onChange={(e) => setNewDateTime(e.target.value)}
              style={{ width: '100%', padding: '10px', borderRadius: '6px', border: '1px solid var(--border)', marginTop: '8px' }}
            />
            {rescheduleError && (
              <p style={{ color: 'var(--color-danger)', fontSize: '0.85rem', marginTop: '8px', marginBottom: 0 }}>
                {rescheduleError}
              </p>
            )}
            <div className="modal-footer">
              <button className="btn-text" onClick={() => { setAppToReschedule(null); setRescheduleError(null); }} disabled={rescheduling}>Cancelar</button>
              <button className="btn-primary" onClick={handleConfirmReschedule} disabled={rescheduling || !newDateTime}>
                {rescheduling ? 'Guardando...' : 'Confirmar Nueva Fecha'}
              </button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog
        isOpen={!!appToCancel}
        variant="danger"
        title="Cancelar cita"
        message={`¿Seguro que quieres cancelar la cita de ${appToCancel?.patient?.firstName || 'este paciente'}? Esta acción no se puede deshacer.`}
        confirmText="Sí, cancelar"
        cancelText="Regresar"
        onConfirm={handleCancelConfirmed}
        onCancel={() => setAppToCancel(null)}
      />
    </div>
  );
};