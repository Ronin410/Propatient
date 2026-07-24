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

export function isIOS(): boolean {
  const ua = window.navigator.userAgent;
  return /iPad|iPhone|iPod/.test(ua) && !(window as unknown as { MSStream?: unknown }).MSStream;
}

// Safari en iOS no dispara "beforeinstallprompt" (Apple no implementa ese
// evento) — ahí no hay forma programática de ofrecer el instalador, solo
// se puede indicarle al usuario el camino manual (Compartir → Agregar a
// inicio), que en iOS SIEMPRE está disponible (a diferencia de Android,
// donde Chrome decide con una heurística interna si lo ofrece o no).
export function isIOSSafari(): boolean {
  const ua = window.navigator.userAgent;
  const isSafari = /Safari/.test(ua) && !/CriOS|FxiOS|EdgiOS/.test(ua);
  return isIOS() && isSafari;
}

// Chrome/Firefox/Edge para iOS (CriOS/FxiOS/EdgiOS en el user agent) usan
// el motor de Safari por dentro (regla de Apple), pero NO ofrecen "Agregar
// a inicio" con soporte completo de PWA (notificaciones push, modo
// standalone) — eso solo funciona si se agrega desde Safari mismo. Sin
// este chequeo, un doctor que entra desde Chrome en su iPhone no ve ni el
// botón de instalar ni ningún aviso — se queda sin saber que existe la opción.
export function isIOSNonSafari(): boolean {
  return isIOS() && !isIOSSafari();
}

export function isAndroid(): boolean {
  return /Android/.test(window.navigator.userAgent);
}
