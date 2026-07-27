import React from 'react';
import { Link } from 'react-router-dom';
import './Footer.scss';

// Misma variable que usa el botón "¿Necesitas ayuda?" del sidebar del
// doctor (ver DashboardLayout.tsx) — antes este link traía un número fijo
// en el código (el personal de quien lo escribió originalmente), sin
// relación con VITE_SUPPORT_WHATSAPP_NUMBER, así que cambiar la variable
// no lo actualizaba. wa.me quiere el número completo en formato
// internacional, sin "+" ni espacios ni guiones.
const supportWhatsAppNumber = (import.meta.env.VITE_SUPPORT_WHATSAPP_NUMBER as string | undefined)?.replace(/[^0-9]/g, '');

export const Footer: React.FC = () => {
  const year = new Date().getFullYear();

  return (
    <footer className="app-footer">
      <p className="app-footer-copyright">© {year} ProPatient Clinic.</p>
      <nav className="app-footer-links">
        <Link to="/privacidad">Aviso de Privacidad</Link>
        <Link to="/terminos">Términos y Condiciones</Link>
        {supportWhatsAppNumber && (
          <a href={`https://wa.me/${supportWhatsAppNumber}`} target="_blank" rel="noopener noreferrer">
            Soporte
          </a>
        )}
      </nav>
    </footer>
  );
};
