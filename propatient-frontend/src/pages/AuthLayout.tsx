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
      backgroundColor: '#ffffff',
    }}>
      {/* 💻 MITAD IZQUIERDA: Panel Visual (SaaS Moderno) */}
      <div style={{
        flex: 1,
        background: 'linear-gradient(135deg, #6a11cb 0%, var(--accent, #aa3bff) 100%)',
        color: '#ffffff',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'between',
        padding: '50px',
        position: 'relative',
      }} className="login-visual-panel">
        
        {/* Marca Superior */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <div style={{ 
            width: '32px', 
            height: '32px', 
            borderRadius: '50%', 
            background: '#ffffff', 
            display: 'flex', 
            alignItems: 'center', 
            justifyContent: 'center', 
            fontWeight: 'bold', 
            color: '#6a11cb' 
          }}>
            P
          </div>
          <span style={{ fontSize: '18px', fontWeight: 700, letterSpacing: '0.5px' }}>ProPatient</span>
        </div>

        {/* Mensaje Central */}
        <div style={{ maxWidth: '440px', margin: 'auto 0' }}>
          <h1 style={{ color: '#ffffff', fontSize: '36px', fontWeight: 800, lineHeight: '1.2', marginBottom: '20px' }}>
            Gestiona tu consultorio digital en un solo lugar.
          </h1>
          <p style={{ color: 'rgba(255, 255, 255, 0.85)', fontSize: '16px', lineHeight: '1.6' }}>
            Simplifica la administración de expedientes clínicos, recetas electrónicas y el control de tus pacientes con seguridad avanzada.
          </p>

          <div style={{ marginTop: '30px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '14px', color: 'rgba(255, 255, 255, 0.9)' }}>
              <span>✓</span> Validación automatizada de Cédula Profesional
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '14px', color: 'rgba(255, 255, 255, 0.9)' }}>
              <span>✓</span> Expedientes clínicos normados y encriptados
            </div>
          </div>
        </div>

        {/* 📑 FOOTER INSTITUCIONAL */}
        <div style={{ 
          display: 'flex', 
          justifyContent: 'space-between', 
          fontSize: '12px', 
          color: 'rgba(255, 255, 255, 0.6)',
          borderTop: '1px solid rgba(255, 255, 255, 0.1)',
          paddingTop: '20px'
        }}>
          <span>© 2026 ProPatient Medical System.</span>
          <div style={{ display: 'flex', gap: '15px' }}>
            <a href="#privacidad" style={{ color: 'inherit', textDecoration: 'underline' }}>Privacidad</a>
            <a href="#soporte" style={{ color: 'inherit', textDecoration: 'underline' }}>Soporte</a>
          </div>
        </div>
      </div>

      {/* 🔐 MITAD DERECHA: Zona de Contenido Dinámico */}
    <div style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        // 🚀 REMOVIDO: justifyContent: 'center' (Esto causaba el desborde superior en image_50a36a.png)
        alignItems: 'center',
        padding: '40px 20px',
        backgroundColor: 'var(--bg-main, #f8f9fa)',
        maxHeight: '100vh',     // Restringe el contenedor al alto de la pantalla
        overflowY: 'auto',      // El scroll se activa correctamente desde el pixel 0
        boxSizing: 'border-box'
      }}>
        <div style={{ 
          width: '100%', 
          maxWidth: '460px', 
          marginTop: 'auto',    // 🚀 Centra perfectamente en pantallas grandes
          marginBottom: 'auto' // Pero colapsa de manera segura a 0 si el contenido es muy alto
        }}>
          {children}
        </div>
      </div>
    </div>
  );
};