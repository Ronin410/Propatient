import React, { useState, useEffect } from 'react';
import api from '../api/axios';
import { getErrorMessage } from '../utils/errorMessage';
import type { WeekSchedule, DayHours } from '../types';
import './WorkingHours.scss';

const DAY_ORDER: (keyof WeekSchedule)[] = [
  'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday',
];

const DAY_LABELS: Record<keyof WeekSchedule, string> = {
  monday: 'Lunes',
  tuesday: 'Martes',
  wednesday: 'Miércoles',
  thursday: 'Jueves',
  friday: 'Viernes',
  saturday: 'Sábado',
  sunday: 'Domingo',
};

const emptyDay = (): DayHours => ({ enabled: false, start: '09:00', end: '18:00', breaks: [] });

const emptySchedule = (): WeekSchedule => ({
  sunday: emptyDay(),
  monday: emptyDay(),
  tuesday: emptyDay(),
  wednesday: emptyDay(),
  thursday: emptyDay(),
  friday: emptyDay(),
  saturday: emptyDay(),
});

// El backend manda "breaks": null (no []) para un día sin descansos
// todavía configurados — comportamiento normal de Go al serializar un
// slice nil. Se normaliza aquí, en un solo lugar, para que el resto del
// componente pueda asumir siempre un arreglo real.
const DAY_KEYS: (keyof WeekSchedule)[] = [
  'sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday',
];
function normalizeSchedule(days: WeekSchedule): WeekSchedule {
  const normalized = { ...days };
  for (const day of DAY_KEYS) {
    normalized[day] = { ...normalized[day], breaks: normalized[day]?.breaks || [] };
  }
  return normalized;
}

export const WorkingHours: React.FC = () => {
  const [schedule, setSchedule] = useState<WeekSchedule>(emptySchedule());
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    api.get('/doctor/schedule')
      .then((res) => {
        if (res.data?.days) setSchedule(normalizeSchedule(res.data.days));
      })
      .catch((err) => setError(getErrorMessage(err, 'No se pudo cargar el horario.')))
      .finally(() => setLoading(false));
  }, []);

  const updateDay = (day: keyof WeekSchedule, changes: Partial<DayHours>) => {
    setSchedule((prev) => ({ ...prev, [day]: { ...prev[day], ...changes } }));
  };

  const addBreak = (day: keyof WeekSchedule) => {
    setSchedule((prev) => ({
      ...prev,
      [day]: { ...prev[day], breaks: [...prev[day].breaks, { start: '14:00', end: '15:00' }] },
    }));
  };

  const updateBreak = (day: keyof WeekSchedule, index: number, changes: Partial<{ start: string; end: string }>) => {
    setSchedule((prev) => {
      const breaks = prev[day].breaks.map((b, i) => (i === index ? { ...b, ...changes } : b));
      return { ...prev, [day]: { ...prev[day], breaks } };
    });
  };

  const removeBreak = (day: keyof WeekSchedule, index: number) => {
    setSchedule((prev) => ({
      ...prev,
      [day]: { ...prev[day], breaks: prev[day].breaks.filter((_, i) => i !== index) },
    }));
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setMessage(null);
    try {
      await api.put('/doctor/schedule', { days: schedule });
      setMessage('Horario guardado. Ya no se podrán agendar citas fuera de estos bloques.');
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo guardar el horario.'));
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="working-hours-loading">Cargando horario...</div>;

  return (
    <div className="working-hours-container">
      <div className="working-hours-card">
        <h2>Horario de Atención</h2>
        <p className="description">
          Define los días y horas en que el consultorio atiende. Fuera de estos
          bloques (y dentro de cualquier descanso que agregues) no se podrán
          agendar citas — ni desde el sistema, ni desde el directorio público.
        </p>

        {error && <div className="working-hours-alert error">{error}</div>}
        {message && <div className="working-hours-alert success">{message}</div>}

        <div className="days-list">
          {DAY_ORDER.map((day) => {
            const dayData = schedule[day];
            return (
              <div className={`day-row ${dayData.enabled ? '' : 'disabled'}`} key={day}>
                <label className="day-toggle">
                  <input
                    type="checkbox"
                    checked={dayData.enabled}
                    onChange={(e) => updateDay(day, { enabled: e.target.checked })}
                  />
                  {DAY_LABELS[day]}
                </label>

                {dayData.enabled && (
                  <div className="day-body">
                    <div className="time-group">
                      <label>Desde</label>
                      <input
                        type="time"
                        value={dayData.start}
                        onChange={(e) => updateDay(day, { start: e.target.value })}
                      />
                      <label>Hasta</label>
                      <input
                        type="time"
                        value={dayData.end}
                        onChange={(e) => updateDay(day, { end: e.target.value })}
                      />
                    </div>

                    <div className="breaks-list">
                      {dayData.breaks.map((brk, idx) => (
                        <div className="break-row" key={idx}>
                          <span className="break-label">No trabaja de</span>
                          <input
                            type="time"
                            value={brk.start}
                            onChange={(e) => updateBreak(day, idx, { start: e.target.value })}
                          />
                          <span className="break-label">a</span>
                          <input
                            type="time"
                            value={brk.end}
                            onChange={(e) => updateBreak(day, idx, { end: e.target.value })}
                          />
                          <button type="button" className="btn-remove-break" onClick={() => removeBreak(day, idx)} aria-label="Quitar descanso">
                            <span className="material-icons-outlined">close</span>
                          </button>
                        </div>
                      ))}
                      <button type="button" className="btn-add-break" onClick={() => addBreak(day)}>
                        <span className="material-icons-outlined">add</span> Agregar descanso
                      </button>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>

        <div className="working-hours-actions">
          <button type="button" className="btn-save-main" onClick={handleSave} disabled={saving}>
            {saving ? 'Guardando...' : 'Guardar Horario'}
          </button>
        </div>
      </div>
    </div>
  );
};
