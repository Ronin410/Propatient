import React, { useState } from 'react';
import { useInstallPrompt } from '../hooks/useInstallPrompt';
import { isIOSNonSafari, isIOSSafari } from '../utils/platformDetection';
import './InstallPwaButton.scss';

interface InstallPwaButtonProps {
  className?: string;
  icon?: string;
}

// Botón para instalar la PWA sin depender de que el usuario encuentre la
// opción en el menú del navegador. Se renderiza distinto según el caso:
// - Chrome/Edge/Samsung Internet cuando ya decidieron que es instalable:
//   botón real que dispara el prompt nativo.
// - iOS Safari: un textito con las instrucciones manuales (no hay API).
// - iOS con otro navegador (Chrome/Firefox/Edge para iOS): esos usan el
//   motor de Safari por dentro pero Apple no les da soporte completo de
//   PWA (notificaciones push, modo standalone) — hay que decirle al
//   usuario que lo abra en Safari, si no, se queda sin ninguna opción.
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

  if (isIOSNonSafari()) {
    return (
      <div className="install-pwa-ios">
        <button type="button" className={`install-pwa-button ${className}`} onClick={() => setShowIOSHint((v) => !v)}>
          <span className="material-icons-outlined">{icon}</span>
          Instalar app
        </button>
        {showIOSHint && (
          <p className="install-pwa-ios-hint">
            En iPhone/iPad, ábrela desde <strong>Safari</strong> para poder instalarla — otros navegadores no lo
            permiten aquí.
          </p>
        )}
      </div>
    );
  }

  return null;
};
