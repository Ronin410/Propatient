// Deja pasar solo dígitos, y opcionalmente un "+" inicial (código de país
// explícito, ej. "+525512345678") — mismo criterio que
// internal/whatsapp.NormalizeE164 en el backend, que interpreta cualquier
// número sin "+" como local de México y le antepone "+52" automáticamente
// al mandar WhatsApp. Se usa en el onChange de los campos de teléfono para
// que no se puedan escribir letras u otros símbolos.
export function sanitizePhoneInput(value: string): string {
  const hasLeadingPlus = value.startsWith('+');
  const digits = value.replace(/\D/g, '');
  return hasLeadingPlus ? `+${digits}` : digits;
}
