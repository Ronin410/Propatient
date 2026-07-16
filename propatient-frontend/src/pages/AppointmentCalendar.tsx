import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalTime, formatToLocalDate, toLocalDateKey } from '../utils/dateFormatter';
import type { Appointment } from '../types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { useAuth } from '../context/AuthContext';
import './AppointmentCalendar.scss';

export const AppointmentCalendar: React.FC = () => {
  const [currentDate, setCurrentDate] = useState(new Date());
  const navigate = useNavigate();
  const { isStaff } = useAuth();
  const [appointments, setAppointments] = useState<Appointment[]>([]);
  const [viewMode, setViewMode] = useState<'month' | 'week'>('month');
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [selectedApp, setSelectedApp] = useState<Appointment | null>(null);
  // Fecha (YYYY-MM-DD) del día que se está viendo en detalle: la vista de
  // mes solo pinta unas pocas citas por celda para no verse amontonada (ver
  // MAX_VISIBLE_EVENTS abajo); al elegir el día se abre este panel con
  // TODAS las citas de esa fecha.
  const [dayViewDate, setDayViewDate] = useState<string | null>(null);

  const MAX_VISIBLE_EVENTS = 3;

  // El personal nunca puede iniciar consultas (/consulta/:id ya está
  // protegida por DoctorOnlyRoute); para citas PENDING del doctor, en vez
  // de ir directo al expediente se pide confirmar antes de iniciar —
  // mismo criterio que el botón "Iniciar Atención" de AppointmentTracking.
  const handleEventClick = (ev: Appointment) => {
    setDayViewDate(null);
    const statusStr = (ev.status || '').toLowerCase();
    if (!isStaff && statusStr === 'pending') {
      setSelectedApp(ev);
    } else {
      navigate(`/pacientes/${ev.patientId}`);
    }
  };

  const toISODate = (date: Date) => {
    const y = date.getFullYear();
    const m = String(date.getMonth() + 1).padStart(2, '0');
    const d = String(date.getDate()).padStart(2, '0');
    return `${y}-${m}-${d}`;
  };

  const getMonthRange = (date: Date) => {
    const start = new Date(date.getFullYear(), date.getMonth(), 1);
    const end = new Date(date.getFullYear(), date.getMonth() + 1, 0);
    return {
      start: toISODate(start),
      end: toISODate(end),
    };
  };

  const getWeekRange = (date: Date) => {
    const currentDay = date.getDay();
    const start = new Date(date);
    start.setDate(date.getDate() - currentDay);
    
    const end = new Date(start);
    end.setDate(start.getDate() + 6);
    
    return {
      start: toISODate(start),
      end: toISODate(end),
    };
  };

  const fetchAppointments = async () => {
    setLoading(true);
    try {
      const range = viewMode === 'month' ? getMonthRange(currentDate) : getWeekRange(currentDate);
      const statusParam = statusFilter ? `&status=${statusFilter}` : '';
      const response = await api.get(`/appointments?start=${range.start}&end=${range.end}${statusParam}`);
      if (response.data) {
        setAppointments(response.data);
      }
    } catch (error) {
      console.error("Error cargando citas del rango calendario:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAppointments();
  }, [currentDate, viewMode, statusFilter]);

  const handlePrev = () => {
    const next = new Date(currentDate);
    if (viewMode === 'month') {
      next.setMonth(currentDate.getMonth() - 1);
    } else {
      next.setDate(currentDate.getDate() - 7);
    }
    setCurrentDate(next);
  };

  const handleNext = () => {
    const next = new Date(currentDate);
    if (viewMode === 'month') {
      next.setMonth(currentDate.getMonth() + 1);
    } else {
      next.setDate(currentDate.getDate() + 7);
    }
    setCurrentDate(next);
  };

  const goToToday = () => {
    setCurrentDate(new Date());
  };

  const getTitle = () => {
    const options: Intl.DateTimeFormatOptions = viewMode === 'month' 
      ? { month: 'long', year: 'numeric' }
      : { month: 'short', day: 'numeric' };
      
    if (viewMode === 'month') {
      return currentDate.toLocaleDateString('es-MX', options).toUpperCase();
    } else {
      const { start } = getWeekRange(currentDate);
      const startDay = new Date(start + 'T00:00:00');
      return `Semana del ${startDay.toLocaleDateString('es-MX', { day: 'numeric', month: 'short' })}`.toUpperCase();
    }
  };

  const renderDays = () => {
    const days = [];
    const todayStr = toISODate(new Date());

    if (viewMode === 'month') {
      const year = currentDate.getFullYear();
      const month = currentDate.getMonth();
      const firstDayIndex = new Date(year, month, 1).getDay();
      const totalDays = new Date(year, month + 1, 0).getDate();

      // Celdas vacías del inicio del mes
      for (let i = 0; i < firstDayIndex; i++) {
        days.push(<div key={`empty-${i}`} className="calendar-day empty" />);
      }

      // Celdas de los días del mes
      for (let day = 1; day <= totalDays; day++) {
        const thisDateStr = `${year}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
        
        const dayEvents = appointments.filter(app => {
          // Fecha en la zona horaria del consultorio, no la fecha UTC cruda
          // (una cita de las 6pm en UTC-7 se guarda como la 1am UTC del día
          // siguiente; usar split('T')[0] la ponía en el día equivocado).
          const appDateRaw = app.appointmentDateTime;
          const appDate = appDateRaw ? toLocalDateKey(appDateRaw) : '';
          return appDate === thisDateStr;
        });

        const isToday = thisDateStr === todayStr;
        const visibleEvents = dayEvents.slice(0, MAX_VISIBLE_EVENTS);
        const hiddenCount = dayEvents.length - visibleEvents.length;

        days.push(
          <div
            key={`day-${day}`}
            className={`calendar-day ${isToday ? 'today' : ''} ${dayEvents.length > 0 ? 'clickable' : ''}`}
            onClick={() => dayEvents.length > 0 && setDayViewDate(thisDateStr)}
          >
            <span className="day-number">{day}</span>
            <div className="day-events">
              {visibleEvents.map(ev => {
                const evDateRaw = ev.appointmentDateTime;
                const patientObj = ev.patient;
                const statusStr = (ev.status || '').toLowerCase();

                return (
                  <div
                    key={ev.id}
                    className={`event-tag ${statusStr === 'pending' ? 'pending' : statusStr === 'completed' ? 'completed' : ''}`}
                    onClick={(e) => { e.stopPropagation(); handleEventClick(ev); }}
                    title={`${formatToLocalTime(evDateRaw)} - ${patientObj?.firstName || 'Paciente'}`}
                  >
                    {formatToLocalTime(evDateRaw)} {patientObj?.firstName || 'Paciente'}
                  </div>
                );
              })}
              {hiddenCount > 0 && (
                <div
                  className="event-more"
                  onClick={(e) => { e.stopPropagation(); setDayViewDate(thisDateStr); }}
                >
                  +{hiddenCount} más
                </div>
              )}
            </div>
          </div>
        );
      }
    } else {
      // VISTA SEMANAL
      const currentDay = currentDate.getDay();
      const startOfWeek = new Date(currentDate);
      startOfWeek.setDate(currentDate.getDate() - currentDay);

      for (let i = 0; i < 7; i++) {
        const dayIter = new Date(startOfWeek);
        dayIter.setDate(startOfWeek.getDate() + i);
        const thisDateStr = toISODate(dayIter);

        const dayEvents = appointments.filter(app => {
          const appDateRaw = app.appointmentDateTime;
          const appDate = appDateRaw ? toLocalDateKey(appDateRaw) : '';
          return appDate === thisDateStr;
        });

        const isToday = thisDateStr === todayStr;

        days.push(
          <div key={`week-day-${i}`} className={`calendar-day week-slot ${isToday ? 'today' : ''}`}>
            <span className="day-number">{dayIter.getDate()} {dayIter.toLocaleDateString('es-MX', { month: 'short' })}</span>
            <div className="day-events">
              {dayEvents.map(ev => {
                const evDateRaw = ev.appointmentDateTime;
                const patientObj = ev.patient;
                const statusStr = (ev.status || '').toLowerCase();

                return (
                  <div 
                    key={ev.id}
                    className={`event-tag ${statusStr === 'pending' ? 'pending' : statusStr === 'completed' ? 'completed' : ''}`}
                    onClick={() => handleEventClick(ev)}
                    title={`${formatToLocalTime(evDateRaw)} - ${patientObj?.firstName || 'Paciente'}`}
                  >
                    {formatToLocalTime(evDateRaw)} - {patientObj?.firstName || 'Paciente'}
                  </div>
                );
              })}
            </div>
          </div>
        );
      }
    }

    return days;
  };

  const dayViewEvents = dayViewDate
    ? appointments
        .filter(app => (app.appointmentDateTime ? toLocalDateKey(app.appointmentDateTime) : '') === dayViewDate)
        .sort((a, b) => (a.appointmentDateTime || '').localeCompare(b.appointmentDateTime || ''))
    : [];

  const dayViewTitle = dayViewDate
    ? new Date(`${dayViewDate}T00:00:00`).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long' })
    : '';

  return (
    <div className="calendar-page-container">
      <header className="calendar-header">
        <div>
          <h1>Agenda Médica</h1>
          <p className="subtitle">Visualiza y controla el flujo de consultas agendadas por mes y semana.</p>
        </div>

        {/* CONTROLES DE NAVEGACIÓN CENTRALES */}
        <div className="calendar-controls">
          <button className="btn-icon" onClick={handlePrev}>
            <span className="material-icons-outlined">chevron_left</span>
          </button>
          <button className="btn-today" onClick={goToToday}>Hoy</button>
          <h2 className="current-month">{getTitle()}</h2>
          <button className="btn-icon" onClick={handleNext}>
            <span className="material-icons-outlined">chevron_right</span>
          </button>
        </div>

        {/* ACCIONES Y FILTROS */}
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
          <div className="view-mode-toggle">
            <button
              className={`toggle-btn ${viewMode === 'month' ? 'active' : ''}`}
              onClick={() => setViewMode('month')}
            >Mes</button>
            <button
              className={`toggle-btn ${viewMode === 'week' ? 'active' : ''}`}
              onClick={() => setViewMode('week')}
            >Semana</button>
          </div>

          <select
            className="status-filter-select"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            aria-label="Filtrar citas por estado"
          >
            <option value="">Todos los estados</option>
            <option value="PENDING">Pendientes</option>
            <option value="COMPLETED">Completadas</option>
            <option value="CANCELLED">Canceladas</option>
            <option value="NOSHOW">No asistió</option>
          </select>

          <button className="btn-submit" onClick={() => navigate('/appointments/new')}>
            <span className="material-icons-outlined">add_task</span>
            Nueva Cita
          </button>
        </div>
      </header>

      {loading ? (
        <div className="loading-state">
          <p>Actualizando calendario médico...</p>
        </div>
      ) : (
        <div className="card-layout">
          <div className="calendar-weekdays">|
            <div>Dom</div><div>Lun</div><div>Mar</div><div>Mié</div><div>Jue</div><div>Vie</div><div>Sáb</div>
          </div>
          <div className="calendar-grid">
            {renderDays()}
          </div>
        </div>
      )}

      <ConfirmDialog
        isOpen={!!selectedApp}
        title="¿Deseas iniciar la consulta?"
        message={`Estás por iniciar la consulta médica y registrar la evolución para: ${selectedApp?.patient?.firstName || ''} ${selectedApp?.patient?.lastName || ''}`}
        confirmText="Confirmar e Iniciar"
        cancelText="Regresar"
        onConfirm={() => {
          if (selectedApp) navigate(`/consulta/${selectedApp.id}`);
          setSelectedApp(null);
        }}
        onCancel={() => setSelectedApp(null)}
      />

      {dayViewDate && (
        <div className="day-view-overlay" onClick={() => setDayViewDate(null)}>
          <div className="day-view-content" onClick={(e) => e.stopPropagation()}>
            <div className="day-view-header">
              <h3>{dayViewTitle}</h3>
              <button className="btn-icon" onClick={() => setDayViewDate(null)} aria-label="Cerrar">
                <span className="material-icons-outlined">close</span>
              </button>
            </div>
            <div className="day-view-list">
              {dayViewEvents.length === 0 ? (
                <p className="day-view-empty">No hay citas este día.</p>
              ) : (
                dayViewEvents.map(ev => {
                  const evDateRaw = ev.appointmentDateTime;
                  const patientObj = ev.patient;
                  const statusStr = (ev.status || '').toLowerCase();

                  return (
                    <div
                      key={ev.id}
                      className={`event-tag ${statusStr === 'pending' ? 'pending' : statusStr === 'completed' ? 'completed' : ''}`}
                      onClick={() => handleEventClick(ev)}
                    >
                      <strong>{formatToLocalTime(evDateRaw)}</strong> — {patientObj?.firstName || 'Paciente'} {patientObj?.lastName || ''}
                    </div>
                  );
                })
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};