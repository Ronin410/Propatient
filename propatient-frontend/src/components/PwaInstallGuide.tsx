import React, { useState } from 'react';
import { useInstallPrompt } from '../hooks/useInstallPrompt';
import { isAndroid, isIOSNonSafari, isIOSSafari, isStandalonePwa } from '../utils/platformDetection';
import './PwaInstallGuide.scss';

const DISMISSED_KEY = 'pwa_install_guide_dismissed';

// Tarjeta de bienvenida que le explica al doctor, con instrucciones
// específicas para su sistema (iOS vs Android), cómo instalar la app y
// por qué le conviene (recibir notificaciones de nuevas solicitudes de
// cita). Existe porque el botón/menú "Instalar app" del navegador NO es
// confiable en todos los dispositivos Android (depende de una heurística
// interna de Chrome que varía por fabricante) — en vez de que el doctor
// tenga que adivinar por qué no le aparece, se lo explicamos directo.
//
// Se muestra una sola vez por navegador/dispositivo: al cerrarla queda
// guardado en localStorage y no vuelve a aparecer ahí (si el doctor la
// cierra en el celular, sigue viéndola en la tablet la primera vez que
// entre desde ahí — es "por dispositivo", no "por cuenta").
export const PwaInstallGuide: React.FC = () => {
  const { canInstall, promptInstall } = useInstallPrompt();
  const [dismissed, setDismissed] = useState(() => localStorage.getItem(DISMISSED_KEY) === 'true');

  const ios = isIOSSafari();
  const iosOtherBrowser = isIOSNonSafari();
  const android = isAndroid();

  // Ya instalada, ya la cerró antes, o no es un dispositivo móvil (en
  // computadora no tiene sentido pedirle que la instale) — no mostrar nada.
  if (dismissed || isStandalonePwa() || (!ios && !iosOtherBrowser && !android)) return null;

  const handleDismiss = () => {
    localStorage.setItem(DISMISSED_KEY, 'true');
    setDismissed(true);
  };

  // Tras ofrecer el prompt nativo (aceptado o no), ya no tiene sentido
  // seguir mostrando la tarjeta en este dispositivo — se cierra sola,
  // igual que si el doctor la hubiera cerrado a mano.
  const handleInstallClick = async () => {
    await promptInstall();
    handleDismiss();
  };

  return (
    <div className="pwa-install-guide">
      <span className="material-icons-outlined pwa-install-guide-icon">install_mobile</span>

      <div className="pwa-install-guide-text">
        <strong>Instala ProPatient Clinic en tu {ios || iosOtherBrowser ? 'iPhone/iPad' : 'celular o tablet'}</strong>
        {iosOtherBrowser ? (
          <p>
            Ábrela desde <strong>Safari</strong> para poder instalarla — en iPhone/iPad, otros navegadores (Chrome,
            Firefox, Edge) no lo permiten.
          </p>
        ) : ios ? (
          <p>
            Toca <strong>Compartir</strong>{' '}
            <span className="material-icons-outlined pwa-install-guide-inline-icon">ios_share</span> abajo en Safari y
            luego <strong>"Agregar a inicio"</strong>. Así puedes activar las notificaciones de nuevas solicitudes de
            cita desde tu Perfil.
          </p>
        ) : canInstall ? (
          <p>Instálala con un toque para recibir notificaciones de nuevas solicitudes de cita en tu pantalla de inicio.</p>
        ) : (
          <p>
            Toca el menú <strong>⋮</strong> de Chrome (arriba a la derecha) y busca <strong>"Instalar app"</strong> o{' '}
            <strong>"Agregar a pantalla de inicio"</strong>. Si no te aparece ninguna de las dos, sigue funcionando
            igual — solo activa las notificaciones desde tu Perfil sin necesidad de instalar nada.
          </p>
        )}
      </div>

      <div className="pwa-install-guide-actions">
        {canInstall && !ios && (
          <button type="button" className="pwa-install-guide-cta" onClick={handleInstallClick}>
            Instalar
          </button>
        )}
        <button type="button" className="pwa-install-guide-close" onClick={handleDismiss} aria-label="Cerrar">
          <span className="material-icons-outlined">close</span>
        </button>
      </div>
    </div>
  );
};
