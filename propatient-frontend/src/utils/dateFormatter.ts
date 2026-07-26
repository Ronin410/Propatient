// Zona horaria del consultorio (Culiacán/Mazatlán, UTC-7). Compartida por
// todos los helpers de este archivo para que la hora/fecha que ve el
// doctor sea siempre la de su consultorio, no la del navegador ni la cruda
// de la base de datos (que siempre guarda en UTC). Si en el futuro cada
// doctor puede configurar su propia zona horaria, este valor debería salir
// de su perfil en vez de estar fijo aquí — ver también
// auth.AppTimeZone en el backend (Go), que usa el mismo valor para los
// correos/WhatsApp automáticos.
export const APP_TIMEZONE = 'America/Mazatlan';

export const formatToLocalTime = (utcDate: string) => {
  if (!utcDate) return '';

  const date = new Date(utcDate);

  return date.toLocaleTimeString('es-MX', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: true,
    timeZone: APP_TIMEZONE,
  });
};

export const formatToLocalDate = (utcDate: string) => {
  const date = new Date(utcDate);
  return date.toLocaleDateString('es-MX', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: APP_TIMEZONE,
  });
};

// Da la fecha (YYYY-MM-DD) de una marca UTC en la zona horaria del
// consultorio, para poder ubicarla en el día correcto de una grilla de
// calendario. Nunca usar utcDate.split('T')[0] para esto: esa es la fecha
// en UTC, no la local — una cita de las 6pm en UTC-7 se guarda como la
// 1am UTC del día siguiente, y el split('T')[0] la mostraba en el día
// equivocado del calendario.
export const toLocalDateKey = (utcDate: string) => {
  if (!utcDate) return '';
  const date = new Date(utcDate);
  // El locale "en-CA" da directo el formato YYYY-MM-DD.
  return date.toLocaleDateString('en-CA', { timeZone: APP_TIMEZONE });
};
