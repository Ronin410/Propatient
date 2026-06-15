export const formatToLocalTime = (utcDate: string) => {
  if (!utcDate) return '';
  
  const date = new Date(utcDate);
  
  return date.toLocaleTimeString('es-MX', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: true,
    // JS detectará automáticamente que estás en Culiacán (UTC-7)
    // Pero puedes forzarlo si quieres asegurar consistencia:
    timeZone: 'America/Mazatlan' 
  });
};

export const formatToLocalDate = (utcDate: string) => {
  const date = new Date(utcDate);
  return date.toLocaleDateString('es-MX', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric'
  });
};