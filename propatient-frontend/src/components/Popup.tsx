import React from 'react';
import './Popup.scss';

interface PopupProps {
  isOpen: boolean;
  type: 'success' | 'error' | 'info' | 'warning';
  title: string;
  message: string;
  buttonText?: string;
  onClose: () => void;
}

export const Popup: React.FC<PopupProps> = ({
  isOpen,
  type,
  title,
  message,
  buttonText = 'Entendido',
  onClose
}) => {
  // Si no está abierto, no renderiza absolutamente nada
  if (!isOpen) return null;

  // Determinar el icono de Material Icons según el tipo
  const getIcon = () => {
    switch (type) {
      case 'success': return 'check_circle';
      case 'error': return 'error';
      case 'warning': return 'warning';
      default: return 'info';
    }
  };

  return (
    <div className="custom-popup-overlay" onClick={onClose}>
      {/* El stopPropagation evita que el popup se cierre si haces clic dentro del recuadro blanco */}
      <div className="custom-popup-content" onClick={(e) => e.stopPropagation()}>
        <span className={`material-icons-outlined popup-icon icon-${type}`}>
          {getIcon()}
        </span>
        <h3>{title}</h3>
        <p>{message}</p>
        <button className={`btn-close-popup btn-${type}`} onClick={onClose}>
          {buttonText}
        </button>
      </div>
    </div>
  );
};