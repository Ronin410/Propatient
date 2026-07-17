import React from 'react';
import { Link } from 'react-router-dom';
import './AuthLayout.scss';

// wa.me quiere el número completo en formato internacional, sin "+" ni
// espacios ni guiones. Mismo número que components/Footer.tsx.
const SUPPORT_WHATSAPP_NUMBER = '526674983913';

interface AuthLayoutProps {
  children: React.ReactNode;
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children }) => {
  const year = new Date().getFullYear();

  return (
    <div className="auth-layout">
      {/* 💻 MITAD IZQUIERDA: Panel Visual Corporativo — se compacta arriba
          del formulario en celular en vez de aplastarlo a un lado (ver
          AuthLayout.scss). */}
      <div className="auth-layout-visual">
        {/* Marca Superior */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <div style={{
            width: '28px',
            height: '28px',
            borderRadius: '50%',
            background: 'rgba(255, 255, 255, 0.2)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontWeight: '600',
            color: '#ffffff',
            fontSize: '13px'
          }}>
            P
          </div>
          <span style={{ fontSize: '18px', fontWeight: 500, letterSpacing: '0.3px' }}>ProPatient</span>
        </div>

        {/* Mensaje Central */}
        <div className="auth-layout-visual-message">
          <h1>Gestiona tu consultorio digital en un solo lugar.</h1>
          <p>Simplifica tu administración de expedientes clínicos, recetas electrónicas y el control de tus pacientes con seguridad avanzada.</p>

          <div className="auth-layout-visual-message-features" style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '14px', color: 'rgba(255, 255, 255, 0.85)' }}>
              <span style={{ color: '#ffffff', fontWeight: 'bold' }}>✓</span> Validación automatizada de Cédula Profesional
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '14px', color: 'rgba(255, 255, 255, 0.85)' }}>
              <span style={{ color: '#ffffff', fontWeight: 'bold' }}>✓</span> Expedientes clínicos normados y encriptados
            </div>
          </div>
        </div>

        {/* 📑 FOOTER INSTITUCIONAL: oculto en celular (ver AuthLayout.scss), para no competir con el formulario por espacio. */}
        <div className="auth-layout-visual-footer">
          <span>© {year} ProPatient Medical System.</span>
          <div style={{ display: 'flex', gap: '20px' }}>
            <Link to="/privacidad" style={{ color: 'inherit', textDecoration: 'none' }}>Privacidad</Link>
            <Link to="/terminos" style={{ color: 'inherit', textDecoration: 'none' }}>Términos</Link>
            <a
              href={`https://wa.me/${SUPPORT_WHATSAPP_NUMBER}`}
              target="_blank"
              rel="noopener noreferrer"
              style={{ color: 'inherit', textDecoration: 'none' }}
            >
              Soporte
            </a>
          </div>
        </div>
      </div>

      {/* 🔐 MITAD DERECHA: Contenedor de la Tarjeta Flotante */}
      <div className="auth-layout-card-container">
        <div className="auth-layout-card">
          {children}
        </div>
      </div>
    </div>
  );
};
