import React from 'react';
import { Link } from 'react-router-dom';
import './Footer.scss';

// wa.me quiere el número completo en formato internacional, sin "+" ni
// espacios ni guiones.
const SUPPORT_WHATSAPP_NUMBER = '526674983913';

export const Footer: React.FC = () => {
  const year = new Date().getFullYear();

  return (
    <footer className="app-footer">
      <p className="app-footer-copyright">© {year} ProPatient Medical System.</p>
      <nav className="app-footer-links">
        <Link to="/privacidad">Aviso de Privacidad</Link>
        <Link to="/terminos">Términos y Condiciones</Link>
        <a href={`https://wa.me/${SUPPORT_WHATSAPP_NUMBER}`} target="_blank" rel="noopener noreferrer">
          Soporte
        </a>
      </nav>
    </footer>
  );
};
