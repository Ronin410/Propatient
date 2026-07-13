import React from 'react';

interface AuthLayoutProps {
  children: React.ReactNode;
}

export const AuthLayout: React.FC<AuthLayoutProps> = ({ children }) => {
  return (
    <div style={{
      display: 'flex',
      minHeight: '100vh',
      width: '100vw',
      backgroundColor: '#e6e4e0', // Fondo crema de la mitad derecha según image_850f73.jpg
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif'
    }}>
      {/* 💻 MITAD IZQUIERDA: Panel Visual Corporativo */}
      <div style={{
        flex: 1,
        background: 'linear-gradient(145deg, #005073 0%, #002d42 100%)', // Azul profundo exacto
        color: '#ffffff',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'space-between',
        padding: '60px 50px',
        position: 'relative',
      }} className="login-visual-panel">
        
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
        <div style={{ maxWidth: '460px', margin: 'auto 0', zIndex: 2 }}>
          <h1 style={{ color: '#ffffff', fontSize: '38px', fontWeight: 600, lineHeight: '1.25', marginBottom: '24px', letterSpacing: '-0.5px' }}>
            Gestiona tu consultorio digital en un solo lugar.
          </h1>
          <p style={{ color: 'rgba(255, 255, 255, 0.75)', fontSize: '15px', lineHeight: '1.6', marginBottom: '32px' }}>
            Simplifica tu administración de expedientes clínicos, recetas electrónicas y el control de tus pacientes con seguridad avanzada.
          </p>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '14px', color: 'rgba(255, 255, 255, 0.85)' }}>
              <span style={{ color: '#ffffff', fontWeight: 'bold' }}>✓</span> Validación automatizada de Cédula Profesional
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '14px', color: 'rgba(255, 255, 255, 0.85)' }}>
              <span style={{ color: '#ffffff', fontWeight: 'bold' }}>✓</span> Expedientes clínicos normados y encriptados
            </div>
          </div>
        </div>

        {/* 📑 FOOTER INSTITUCIONAL */}
        <div style={{ 
          display: 'flex', 
          justifyContent: 'space-between', 
          fontSize: '12px', 
          color: 'rgba(255, 255, 255, 0.5)',
          borderTop: '1px solid rgba(255, 255, 255, 0.08)',
          paddingTop: '20px',
          zIndex: 2
        }}>
          <span>© 2026 ProPatient Medical System.</span>
          <div style={{ display: 'flex', gap: '20px' }}>
            <a href="#privacidad" style={{ color: 'inherit', textDecoration: 'none' }}>Privacidad</a>
            <a href="#soporte" style={{ color: 'inherit', textDecoration: 'none' }}>Soporte</a>
          </div>
        </div>
      </div>

      {/* 🔐 MITAD DERECHA: Contenedor de la Tarjeta Flotante */}
      <div style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '40px 20px',
        backgroundColor: '#e6e4e0', // Fondo arena unificado
        maxHeight: '100vh',
        overflowY: 'auto',
        boxSizing: 'border-box'
      }}>
        <div style={{ 
          width: '100%', 
          maxWidth: '410px', // Ancho óptimo idéntico para la tarjeta flotante
          position: 'relative',
          paddingTop: '40px' // Espacio para que el escudo sobresalga
        }}>
          {children}
        </div>
      </div>
    </div>
  );
};