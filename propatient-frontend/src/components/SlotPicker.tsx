import React from 'react';
import type { WeekSchedule } from '../types';
import { computeAvailableSlots, nowInAppTimezone } from '../utils/appointmentSlots';
import './SlotPicker.scss';

interface SlotPickerProps {
  // El llamador decide si mostrar este componente o el <input
  // type="datetime-local"> libre de siempre — solo tiene sentido cuando el
  // doctor sí configuró un horario (ver WorkingHours.tsx).
  schedule: WeekSchedule;
  dateKey: string;
  onDateChange: (dateKey: string) => void;
  selectedTime: string | null;
  onSelectTime: (time: string) => void;
}

// "HH:MM" (24h) a "hh:mm a.m./p.m." — mismo estilo que formatToLocalTime.
function formatSlotLabel(time: string): string {
  const [h, m] = time.split(':').map(Number);
  const d = new Date(2000, 0, 1, h, m);
  return d.toLocaleTimeString('es-MX', { hour: 'numeric', minute: '2-digit', hour12: true });
}

// Selector de horario en incrementos de 30 minutos, a partir del horario
// laboral configurado por el doctor (ver appointmentSlots.ts) — reemplaza
// el <input type="datetime-local"> libre que antes se usaba tanto en el
// agendado interno como en el formulario público, sin ninguna guía de qué
// horarios tiene disponibles el consultorio.
export const SlotPicker: React.FC<SlotPickerProps> = ({ schedule, dateKey, onDateChange, selectedTime, onSelectTime }) => {
  const { dateKey: todayKey } = nowInAppTimezone();
  const slots = computeAvailableSlots(schedule, dateKey);

  return (
    <div className="slot-picker">
      <input
        type="date"
        className="slot-picker-date"
        value={dateKey}
        min={todayKey}
        onChange={(e) => onDateChange(e.target.value)}
        required
      />

      {dateKey && slots.length === 0 && (
        <p className="slot-picker-empty">El consultorio no atiende ese día. Elige otra fecha.</p>
      )}

      {slots.length > 0 && (
        <div className="slot-picker-grid">
          {slots.map((slot) => (
            <button
              key={slot.time}
              type="button"
              className={`slot-btn ${selectedTime === slot.time ? 'selected' : ''}`}
              disabled={!slot.available}
              title={!slot.available ? 'Ese horario ya pasó por hoy' : undefined}
              onClick={() => onSelectTime(slot.time)}
            >
              {formatSlotLabel(slot.time)}
            </button>
          ))}
        </div>
      )}
    </div>
  );
};
