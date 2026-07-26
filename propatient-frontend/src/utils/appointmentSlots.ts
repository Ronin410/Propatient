// Genera los horarios agendables de un día en incrementos de 30 minutos a
// partir del horario laboral configurado por el doctor (ver WeekSchedule).
// Usado tanto por el agendado interno (AppointmentForm) como por el
// formulario público de citas (PublicDoctorProfile) — antes ambos usaban
// un <input type="datetime-local"> libre, sin ninguna guía de qué horarios
// tiene disponibles el consultorio.
//
// A propósito NO excluye horarios que ya tengan una cita: el consultorio
// puede agendar más de un paciente en el mismo horario si así lo decide
// (ver ExecNightClosure en el backend para el criterio equivalente del
// lado de "cuándo se considera vencida" una cita, mismo espíritu de no
// sobre-restringir). Lo único que un horario de HOY marca como no
// disponible es que ya haya pasado (con 30 min de margen) — los días
// futuros no tienen ninguna restricción de hora.
import type { WeekSchedule } from '../types';
import { APP_TIMEZONE } from './dateFormatter';

export const SLOT_INTERVAL_MINUTES = 30;
const TODAY_BOOKING_LEAD_MINUTES = 30;

const WEEKDAY_KEYS: (keyof WeekSchedule)[] = [
  'sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday',
];

export interface TimeSlot {
  /** "HH:MM" en hora local del consultorio. */
  time: string;
  /** Minutos desde medianoche, para ordenar/comparar. */
  minutes: number;
  available: boolean;
}

function parseHHMM(value: string | undefined): number | null {
  if (!value) return null;
  const match = /^(\d{1,2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const h = Number(match[1]);
  const m = Number(match[2]);
  if (h < 0 || h > 23 || m < 0 || m > 59) return null;
  return h * 60 + m;
}

function minutesToHHMM(totalMinutes: number): string {
  const h = Math.floor(totalMinutes / 60).toString().padStart(2, '0');
  const m = (totalMinutes % 60).toString().padStart(2, '0');
  return `${h}:${m}`;
}

/** "YYYY-MM-DD" y minutos desde medianoche de "ahora" en APP_TIMEZONE. */
export function nowInAppTimezone(): { dateKey: string; minutes: number } {
  const now = new Date();
  const dateKey = now.toLocaleDateString('en-CA', { timeZone: APP_TIMEZONE });
  const timeStr = now.toLocaleTimeString('en-GB', {
    timeZone: APP_TIMEZONE,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
  const [h, m] = timeStr.split(':').map(Number);
  return { dateKey, minutes: h * 60 + m };
}

/**
 * Convierte una hora de pared ("YYYY-MM-DD" + "HH:MM") en la zona horaria
 * dada a la marca UTC real correspondiente. Necesario porque JS no tiene
 * una forma directa de construir un Date a partir de una hora local en UNA
 * zona horaria específica (distinta de la del navegador) — usa el truco
 * estándar de doble conversión vía Intl para que funcione correctamente
 * incluso en transiciones de horario de verano.
 */
export function zonedTimeToUtc(dateKey: string, hhmm: string, timeZone: string): Date {
  const [y, m, d] = dateKey.split('-').map(Number);
  const [hh, mm] = hhmm.split(':').map(Number);

  // Primera aproximación: tratamos la hora deseada como si ya fuera UTC.
  const guessUtc = new Date(Date.UTC(y, m - 1, d, hh, mm, 0));

  // ¿Qué hora de pared muestra esa marca en la zona horaria destino?
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone,
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
    hour12: false,
  }).formatToParts(guessUtc);
  const get = (type: string) => Number(parts.find((p) => p.type === type)?.value ?? 0);
  // Algunos motores devuelven "24" para medianoche con hour12:false.
  const hour = get('hour') % 24;
  const shownAsUtc = Date.UTC(get('year'), get('month') - 1, get('day'), hour, get('minute'), get('second'));

  const offset = guessUtc.getTime() - shownAsUtc;
  return new Date(guessUtc.getTime() + offset);
}

/**
 * Genera los horarios de 30 en 30 minutos dentro de la ventana laboral del
 * doctor para dateKey ("YYYY-MM-DD"), excluyendo descansos. Si dateKey es
 * el día de hoy (en APP_TIMEZONE), los horarios anteriores a "ahora - 30
 * min" quedan marcados como no disponibles; cualquier otro día no tiene
 * esa restricción. Si el doctor no configuró horario, o ese día no
 * atiende, devuelve un arreglo vacío.
 */
export function computeAvailableSlots(schedule: WeekSchedule | null | undefined, dateKey: string): TimeSlot[] {
  if (!schedule || !dateKey) return [];

  const [y, m, d] = dateKey.split('-').map(Number);
  if (!y || !m || !d) return [];

  const weekdayIndex = new Date(Date.UTC(y, m - 1, d)).getUTCDay(); // 0=domingo
  const day = schedule[WEEKDAY_KEYS[weekdayIndex]];
  if (!day || !day.enabled) return [];

  const startMin = parseHHMM(day.start);
  const endMin = parseHHMM(day.end);
  if (startMin === null || endMin === null || startMin >= endMin) return [];

  const breaks = (day.breaks || [])
    .map((b) => ({ start: parseHHMM(b.start), end: parseHHMM(b.end) }))
    .filter((b): b is { start: number; end: number } => b.start !== null && b.end !== null);

  const { dateKey: todayKey, minutes: nowMinutes } = nowInAppTimezone();
  const isToday = dateKey === todayKey;
  const cutoff = nowMinutes - TODAY_BOOKING_LEAD_MINUTES;

  const slots: TimeSlot[] = [];
  for (let t = startMin; t < endMin; t += SLOT_INTERVAL_MINUTES) {
    const inBreak = breaks.some((b) => t >= b.start && t < b.end);
    if (inBreak) continue;
    slots.push({
      time: minutesToHHMM(t),
      minutes: t,
      available: !isToday || t >= cutoff,
    });
  }
  return slots;
}
