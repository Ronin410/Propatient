// Ayuda para Google reCAPTCHA v3 en formularios públicos sensibles a spam
// (agendar cita sin cuenta). Sin VITE_RECAPTCHA_SITE_KEY configurada, todas
// las funciones de aquí son no-ops — el formulario sigue funcionando igual,
// solo sin esta capa extra de protección (el backend ya tiene su propio
// rate limiting activo siempre, ver internal/middleware).

declare global {
  interface Window {
    grecaptcha?: {
      ready: (cb: () => void) => void;
      execute: (siteKey: string, options: { action: string }) => Promise<string>;
    };
  }
}

const SITE_KEY = import.meta.env.VITE_RECAPTCHA_SITE_KEY as string | undefined;

let scriptLoadPromise: Promise<void> | null = null;

function loadScript(): Promise<void> {
  if (!SITE_KEY) return Promise.resolve();
  if (scriptLoadPromise) return scriptLoadPromise;

  scriptLoadPromise = new Promise((resolve) => {
    const existing = document.querySelector('script[data-recaptcha-v3]');
    if (existing) {
      resolve();
      return;
    }
    const script = document.createElement('script');
    script.src = `https://www.google.com/recaptcha/api.js?render=${SITE_KEY}`;
    script.async = true;
    script.dataset.recaptchaV3 = 'true';
    script.onload = () => resolve();
    // Si el script falla en cargar (bloqueado por un ad-blocker, sin red,
    // etc.) no debe tumbar el formulario — se resuelve igual y
    // getRecaptchaToken() simplemente devuelve undefined.
    script.onerror = () => resolve();
    document.body.appendChild(script);
  });
  return scriptLoadPromise;
}

// Se llama una vez, idealmente al montar el formulario, para que el script
// ya esté listo cuando el usuario le dé "enviar" (evita el salto extra de
// red justo en el momento del submit).
export function preloadRecaptcha(): void {
  void loadScript();
}

// Devuelve un token de reCAPTCHA v3 para la acción dada, o undefined si no
// está configurado o algo falló — el backend trata la ausencia de token
// igual que "no configurado" (ver internal/recaptcha/verify.go).
export async function getRecaptchaToken(action: string): Promise<string | undefined> {
  if (!SITE_KEY) return undefined;
  await loadScript();
  if (!window.grecaptcha) return undefined;

  try {
    return await new Promise<string>((resolve, reject) => {
      window.grecaptcha!.ready(() => {
        window.grecaptcha!.execute(SITE_KEY, { action }).then(resolve, reject);
      });
    });
  } catch {
    return undefined;
  }
}
