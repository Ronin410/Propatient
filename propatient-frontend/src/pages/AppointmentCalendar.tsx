import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalTime, formatToLocalDate } from '../utils/dateFormatter';
import type { Appointment } from '../types'; 
import './AppointmentCalendar.scss';

export const AppointmentCalendar: React.FC = () => {
  const [currentDate, setCurrentDate] = useState(new Date());
  const navigate = useNavigate();
  const [appointments, setAppointments] = useState<Appointment[]>([]);
  const [viewMode, setViewMode] = useState<'month' | 'week'>('month');
  const [loading, setLoading] = useState(false);

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
      const response = await api.get(`/appointments?start=${range.start}&end=${range.end}`);
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
  }, [currentDate, viewMode]);

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
          // Extraemos la fecha resolviendo de manera segura las diferentes nomenclaturas posibles
          const appDateRaw = app.appointmentDateTime || app.appointment_date || app.appointmentDate || app.AppointmentDate;
          const appDate = appDateRaw ? appDateRaw.split('T')[0] : '';
          return appDate === thisDateStr;
        });

        const isToday = thisDateStr === todayStr;

        days.push(
          <div key={`day-${day}`} className={`calendar-day ${isToday ? 'today' : ''}`}>
            <span className="day-number">{day}</span>
            <div className="day-events">
              {dayEvents.map(ev => {
                const evDateRaw = ev.appointmentDateTime || ev.appointment_date || ev.appointmentDate || ev.AppointmentDate;
                const patientObj = ev.patient || ev.Patient;
                const statusStr = (ev.status || ev.Status || '').toLowerCase();

                return (
                  <div 
                    key={ev.id || ev.ID} 
                    className={`event-tag ${statusStr === 'pending' ? 'pending' : statusStr === 'completed' ? 'completed' : ''}`}
                    onClick={() => navigate(`/pacientes/${ev.patient_id || ev.patientId || ev.PatientId}`)}
                    title={`${formatToLocalTime(evDateRaw)} - ${patientObj?.firstName || 'Paciente'}`}
                  >
                    {formatToLocalTime(evDateRaw)} {patientObj?.firstName || 'Paciente'}
                  </div>
                );
              })}
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
          const appDateRaw = app.appointmentDateTime || app.appointment_date || app.appointmentDate || app.AppointmentDate;
          const appDate = appDateRaw ? appDateRaw.split('T')[0] : '';
          return appDate === thisDateStr;
        });

        const isToday = thisDateStr === todayStr;

        days.push(
          <div key={`week-day-${i}`} className={`calendar-day week-slot ${isToday ? 'today' : ''}`}>
            <span className="day-number">{dayIter.getDate()} {dayIter.toLocaleDateString('es-MX', { month: 'short' })}</span>
            <div className="day-events">
              {dayEvents.map(ev => {
                const evDateRaw = ev.appointmentDateTime || ev.appointment_date || ev.appointmentDate || ev.AppointmentDate;
                const patientObj = ev.patient || ev.Patient;
                const statusStr = (ev.status || ev.Status || '').toLowerCase();

                return (
                  <div 
                    key={ev.id || ev.ID} 
                    className={`event-tag ${statusStr === 'pending' ? 'pending' : statusStr === 'completed' ? 'completed' : ''}`}
                    onClick={() => navigate(`/pacientes/${ev.patient_id || ev.patientId || ev.PatientId}`)}
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
    </div>
  );
};