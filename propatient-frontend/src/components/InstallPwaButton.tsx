import React, { useState } from 'react';
import { useInstallPrompt } from '../hooks/useInstallPrompt';
import './InstallPwaButton.scss';

// Safari en iOS no dispara "beforeinstallprompt" (Apple no implementa ese
// evento) — ahí no hay forma programática de ofrecer el instalador, solo
// se puede indicarle al usuario el camino manual (Compartir → Agregar a
// inicio).
function isIOSSafari(): boolean {
  const ua = window.navigator.userAgent;
  const isIOS = /iPad|iPhone|iPod/.test(ua) && !(window as unknown as { MSStream?: unknown }).MSStream;
  const isSafari = /Safari/.test(ua) && !/CriOS|FxiOS|EdgiOS/.test(ua);
  return isIOS && isSafari;
}

interface InstallPwaButtonProps {
  className?: string;
  icon?: string;
}

// Botón para instalar la PWA sin depender de que el usuario encuentre la
// opción en el menú del navegador. Se renderiza distinto según el caso:
// - Chrome/Edge/Samsung Internet cuando ya decidieron que es instalable:
//   botón real que dispara el prompt nativo.
// - iOS Safari: un textito con las instrucciones manuales (no hay API).
// - Cualquier otro caso (ya instalada, o el navegador no lo decidió
//   todavía): no renderiza nada, para no ensuciar la UI con un botón que
//   no haría nada.
export const InstallPwaButton: React.FC<InstallPwaButtonProps> = ({ className = '', icon = 'install_mobile' }) => {
  const { canInstall, promptInstall } = useInstallPrompt();
  const [showIOSHint, setShowIOSHint] = useState(false);

  if (canInstall) {
    return (
      <button type="button" className={`install-pwa-button ${className}`} onClick={promptInstall}>
        <span className="material-icons-outlined">{icon}</span>
        Instalar app
      </button>
    );
  }

  if (isIOSSafari()) {
    return (
      <div className="install-pwa-ios">
        <button type="button" className={`install-pwa-button ${className}`} onClick={() => setShowIOSHint((v) => !v)}>
          <span className="material-icons-outlined">{icon}</span>
          Instalar app
        </button>
        {showIOSHint && (
          <p className="install-pwa-ios-hint">
            Toca <strong>Compartir</strong> (el ícono con la flecha hacia arriba) y luego{' '}
            <strong>"Agregar a inicio"</strong>.
          </p>
        )}
      </div>
    );
  }

  return null;
};
