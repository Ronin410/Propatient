// Helpers de detección de plataforma para todo lo relacionado a instalar
// la PWA — centralizados aquí para que useInstallPrompt, InstallPwaButton
// y PwaInstallGuide usen exactamente el mismo criterio, sin duplicar la
// lógica en cada archivo.

export function isStandalonePwa(): boolean {
  return (
    window.matchMedia('(display-mode: standalone)').matches ||
    // iOS Safari no tiene la media query de arriba, usa esta propiedad no
    // estándar en su lugar.
    (window.navigator as Navigator & { standalone?: boolean }).standalone === true
  );
}

// Safari en iOS no dispara "beforeinstallprompt" (Apple no implementa ese
// evento) — ahí no hay forma programática de ofrecer el instalador, solo
// se puede indicarle al usuario el camino manual (Compartir → Agregar a
// inicio), que en iOS SIEMPRE está disponible (a diferencia de Android,
// donde Chrome decide con una heurística interna si lo ofrece o no).
export function isIOSSafari(): boolean {
  const ua = window.navigator.userAgent;
  const isIOS = /iPad|iPhone|iPod/.test(ua) && !(window as unknown as { MSStream?: unknown }).MSStream;
  const isSafari = /Safari/.test(ua) && !/CriOS|FxiOS|EdgiOS/.test(ua);
  return isIOS && isSafari;
}

export function isAndroid(): boolean {
  return /Android/.test(window.navigator.userAgent);
}
