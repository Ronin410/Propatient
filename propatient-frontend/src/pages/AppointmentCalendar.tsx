import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalTime, formatToLocalDate } from '../utils/dateFormatter';
import type { Appointment, Patient } from '../types'; 
import './AppointmentCalendar.scss';

export const AppointmentCalendar: React.FC = () => {
  const [currentDate, setCurrentDate] = useState(new Date());
  const navigate = useNavigate();
  const [appointments, setAppointments] = useState<Appointment[]>([]);
  const [viewMode, setViewMode] = useState<'month' | 'week'>('month');
  const [loading, setLoading] = useState(false);

  // Helper para obtener YYYY-MM-DD sin desfases de UTC
  const toISODate = (date: Date) => {
    const y = date.getFullYear();
    const m = String(date.getMonth() + 1).padStart(2, '0');
    const d = String(date.getDate()).padStart(2, '0');
    return `${y}-${m}-${d}`;
  };

  // Obtener el primer y último día del mes actual para la API
  const getMonthRange = (date: Date) => {
    const start = new Date(date.getFullYear(), date.getMonth(), 1);
    const end = new Date(date.getFullYear(), date.getMonth() + 1, 0);
    return {
      start: toISODate(start),
      end: toISODate(end)
    };
  };

  // Obtener el rango de la semana actual (Dom a Sáb)
  const getWeekRange = (date: Date) => {
    const start = new Date(date);
    const day = start.getDay();
    const diff = start.getDate() - day;
    const weekStart = new Date(start.setDate(diff));
    const weekEnd = new Date(weekStart);
    weekEnd.setDate(weekStart.getDate() + 6);
    return {
      start: toISODate(weekStart),
      end: toISODate(weekEnd)
    };
  };

  const fetchAppointments = async () => {
    setLoading(true);
    const { start, end } = viewMode === 'month' ? getMonthRange(currentDate) : getWeekRange(currentDate);
    try {
      // Tu handler en Go acepta ?start=...&end=...
      const response = await api.get(`/appointments?start=${start}&end=${end}`);
      setAppointments(response.data || []);
    } catch (error) {
      console.error("Error al cargar citas del calendario:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAppointments();
  }, [currentDate, viewMode]);

  const handleNext = () => {
    if (viewMode === 'month') {
      setCurrentDate(new Date(currentDate.getFullYear(), currentDate.getMonth() + 1, 1));
    } else {
      const nextWeek = new Date(currentDate);
      nextWeek.setDate(currentDate.getDate() + 7);
      setCurrentDate(nextWeek);
    }
  };

  const handlePrev = () => {
    if (viewMode === 'month') {
      setCurrentDate(new Date(currentDate.getFullYear(), currentDate.getMonth() - 1, 1));
    } else {
      const prevWeek = new Date(currentDate);
      prevWeek.setDate(currentDate.getDate() - 7);
      setCurrentDate(prevWeek);
    }
  };

  const goToToday = () => setCurrentDate(new Date());

  const getTitle = () => {
    if (viewMode === 'month') {
      return currentDate.toLocaleString('es-ES', { month: 'long', year: 'numeric' });
    }
    const { start, end } = getWeekRange(currentDate);
    const startD = new Date(start + "T00:00:00");
    const endD = new Date(end + "T00:00:00");
    return `${startD.getDate()} - ${endD.getDate()} ${startD.toLocaleString('es-ES', { month: 'short' })}, ${startD.getFullYear()}`;
  };

  const renderDaySlot = (dayNumber: number, dateStr: string) => {
    const dayAppointments = appointments.filter(app => app.appointmentDateTime?.startsWith(dateStr));
    const isToday = toISODate(new Date()) === dateStr;
    
    return (
      <div key={dateStr} className={`calendar-day ${isToday ? 'today' : ''} ${viewMode === 'week' ? 'week-slot' : ''}`}>
        <span className="day-number">{dayNumber}</span>
        <div className="day-events">
          {Array.isArray(dayAppointments) && dayAppointments.map(app => (
            <div key={app.id} className={`event-tag ${app.status.toLowerCase()}`}>
              <span className="event-time">{formatToLocalTime(app.appointmentDateTime)}</span>
              <span className="event-patient">
                {app.patient?.firstName} {app.patient?.lastName?.charAt(0)}.
              </span>
            </div>
          ))}
        </div>
      </div>
    );
  };

  const renderDays = () => {
    const year = currentDate.getFullYear();
    const month = currentDate.getMonth();
    const days = [];

    if (viewMode === 'month') {
      const firstDayOfMonth = new Date(year, month, 1).getDay();
      const daysInMonth = new Date(year, month + 1, 0).getDate();
      
      for (let i = 0; i < firstDayOfMonth; i++) {
        days.push(<div key={`empty-${i}`} className="calendar-day empty"></div>);
      }

      for (let d = 1; d <= daysInMonth; d++) {
        const dateStr = `${year}-${String(month + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
        days.push(renderDaySlot(d, dateStr));
      }
    } else {
      const { start } = getWeekRange(currentDate);
      const startDate = new Date(start + "T00:00:00");
      for (let i = 0; i < 7; i++) {
        const dayDate = new Date(startDate);
        dayDate.setDate(startDate.getDate() + i);
        days.push(renderDaySlot(dayDate.getDate(), toISODate(dayDate)));
      }
    }
    return days;
  };

  return (
    <div className="calendar-page-container">
      <header className="calendar-header">
        <div className="header-left">
          <h1>Agenda de Citas</h1>
        </div>
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
        <button className="btn-primary" onClick={() => navigate('/appointments/new')}>
          + Nueva Cita
        </button>
      </header>

      {loading ? (
        <div className="loading-state">Actualizando calendario...</div>
      ) : (
        <div className="calendar-card card">
          <div className="calendar-weekdays">
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