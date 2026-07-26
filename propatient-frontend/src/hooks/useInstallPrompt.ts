import { useCallback, useEffect, useState } from 'react';
import { isStandalonePwa } from '../utils/platformDetection';

// Chrome/Edge/Samsung Internet disparan este evento cuando consideran que
// el sitio cumple los requisitos para instalarse (manifest + service
// worker válidos) — no es un tipo estándar de TS todavía, se declara a mano.
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>;
}

interface UseInstallPromptResult {
  // true solo cuando el navegador YA decidió que el sitio es instalable y
  // todavía no se instaló — mostrar un botón antes de esto no serviría de
  // nada (el navegador rechazaría el prompt).
  canInstall: boolean;
  promptInstall: () => Promise<void>;
}

// Permite ofrecer un botón propio de "Instalar app" en vez de depender de
// que el usuario encuentre la opción en el menú del navegador (que en
// Chrome de Android no siempre aparece de inmediato — depende de una
// heurística de "compromiso" con el sitio que Google no documenta del
// todo). Solo funciona en navegadores Chromium; en iOS Safari no existe
// este evento en absoluto, así que canInstall se queda en false ahí
// siempre (ver el aviso alterno para iOS en InstallPwaButton.tsx).
export function useInstallPrompt(): UseInstallPromptResult {
  const [deferredEvent, setDeferredEvent] = useState<BeforeInstallPromptEvent | null>(null);
  const [installed, setInstalled] = useState(isStandalonePwa());

  useEffect(() => {
    const handleBeforeInstallPrompt = (e: Event) => {
      e.preventDefault();
      setDeferredEvent(e as BeforeInstallPromptEvent);
    };
    const handleAppInstalled = () => {
      setInstalled(true);
      setDeferredEvent(null);
    };
    window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt);
    window.addEventListener('appinstalled', handleAppInstalled);
    return () => {
      window.removeEventListener('beforeinstallprompt', handleBeforeInstallPrompt);
      window.removeEventListener('appinstalled', handleAppInstalled);
    };
  }, []);

  const promptInstall = useCallback(async () => {
    if (!deferredEvent) return;
    await deferredEvent.prompt();
    const choice = await deferredEvent.userChoice;
    if (choice.outcome === 'accepted') setInstalled(true);
    setDeferredEvent(null);
  }, [deferredEvent]);

  return { canInstall: !!deferredEvent && !installed, promptInstall };
}
