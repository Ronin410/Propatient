import React, { useState, useEffect } from 'react';
import { useAuth } from '../context/AuthContext';
import api from '../api/axios';
import { useNavigate } from 'react-router-dom';
import { AuthLayout } from './AuthLayout'; 
import './Login.scss'; 

export const Login = () => {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const { login } = useAuth();
  const navigate = useNavigate();

  // Función encargada de inyectar y dar ancho exacto al botón oficial de Google
  const renderGoogleButton = () => {
    /* global google */
    if (typeof google !== 'undefined') {
      try {
        google.accounts.id.initialize({
          client_id: "744896665247-hrs7pmi72q2pog5dn9h1fh5n4nkpv18r.apps.googleusercontent.com",
          callback: handleGoogleResponse,
        });

        const container = document.getElementById("googleBtn");
        if (container) {
          container.innerHTML = ""; // Limpia residuos HTML previos para evitar duplicados
          
          google.accounts.id.renderButton(
            container,
            { 
              theme: "outline", 
              size: "large", 
              text: "continue_with", // "Continuar con Google" alineado a la nueva referencia visual
              width: "340"           // Ancho perfecto balanceado para el padding de la tarjeta
            }
          );
        }
      } catch (err) {
        console.error("Error al renderizar el botón de Google:", err);
      }
    } else {
      console.error("El script de Google Identity Services no se ha cargado en index.html");
    }
  };

  useEffect(() => {
    renderGoogleButton();
    // Re-renderizado de seguridad por si el script tarda milisegundos extra en cargar
    const timer = setTimeout(renderGoogleButton, 150);
    return () => clearTimeout(timer);
  }, [isLoading]);

  const handleGoogleResponse = async (response: any) => {
    setIsLoading(true);
    setError(null);

    try {
      const res = await api.post('/auth/google-login', { 
        idToken: response.credential 
      });
      
      const { perfilCompletado, cedulaValidada } = res.data.userStatus;

      if (res.data.fullName) {
        localStorage.setItem('suggested_fullname', res.data.fullName);
      }

      if (!perfilCompletado) {
        login(res.data.token, { profileCompleted: perfilCompletado, cedulaValidated: cedulaValidada });
        navigate('/registro/perfil');
      } else if (cedulaValidada === 'PENDIENTE') {
        login(res.data.token, { profileCompleted: perfilCompletado, cedulaValidated: cedulaValidada });
        navigate('/registro/validar-cedula');
      } else if (cedulaValidada === 'CAPTURADA') {
        alert("Tu cuenta y cédula profesional están siendo validadas por nuestro equipo técnico. Te notificaremos por correo una vez concluido el proceso.");
        setIsLoading(false); 
      } else if (cedulaValidada === 'VALIDADA') {
        login(res.data.token, { profileCompleted: perfilCompletado, cedulaValidated: cedulaValidada });
        navigate('/inicio');
      }

    } catch (err: any) {
      setError(err.response?.data?.error || 'Error al autenticar con Google. Intente de nuevo.');
      setIsLoading(false);
    }
  };

  return (
    <AuthLayout>
      {/* 🛡️ ESCUDO / LOGO FLOTANTE SUPERIOR */}
      <div style={{
        width: '110px',
        height: '110px',
        backgroundColor: '#007370', // Color verde azulado / Teal corporativo exacto
        borderRadius: '50%',
        position: 'absolute',
        top: '-15px',
        left: '50%',
        transform: 'translateX(-50%)',
        zIndex: 10,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        boxShadow: '0 8px 20px rgba(0, 115, 112, 0.15)'
      }}>
        {/* Isotipo médico minimalista en SVG integrado */}
        <svg width="56" height="56" viewBox="0 0 24 24" fill="none" stroke="#ffffff" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
          <path d="M12 8v8M9 12h6"/>
        </svg>
      </div>

      {/* 📄 TARJETA CONTENEDORA BLANCA */}
      <div className="card" style={{ 
        padding: '80px 35px 40px 35px', // Margen superior amplio para dar espacio al escudo flotante
        backgroundColor: '#f5f4f0',    // Color hueso/crema suave para aislar el contenedor del fondo de la pantalla
        borderRadius: '24px',          // Bordes redondeados modernos y orgánicos
        boxShadow: '0 20px 40px rgba(0, 0, 0, 0.04), 0 1px 3px rgba(0, 0, 0, 0.02)',
        boxSizing: 'border-box',
        width: '100%',
        position: 'relative'
      }}>
        
        {/* Encabezado */}
        <div style={{ textAlign: 'center', marginBottom: '32px' }}>
          <h1 style={{ fontSize: '26px', fontWeight: 700, color: '#0c1017', margin: 0, letterSpacing: '-0.5px' }}>
            Acceso al Sistema
          </h1>
          <p style={{ fontSize: '14px', color: '#535865', marginTop: '6px', fontWeight: 500 }}>
            ProPatient Medical System
          </p>
        </div>

        <div className="login-form">
          {error && (
            <div className="alert alert-danger" style={{ 
              marginBottom: '1.5rem', 
              padding: '12px 14px', 
              borderRadius: '8px', 
              backgroundColor: '#fee2e2', 
              color: '#b91c1c', 
              display: 'flex', 
              alignItems: 'center', 
              gap: '10px', 
              fontSize: '13px',
              border: '1px solid #fca5a5'
            }}>
              <span>⚠️</span>
              <span style={{ fontWeight: 500 }}>{error}</span>
            </div>
          )}

          {isLoading ? (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', padding: '20px 0', gap: '14px' }}>
              <div className="spinner-border" role="status" style={{ color: '#007370' }}></div>
              <p style={{ fontSize: '14px', color: '#535865', fontWeight: 500 }}>Validando identidad médica...</p>
            </div>
          ) : (
            <>
              {/* Contenedor donde la API de Google inyectará el botón nativo */}
              <div style={{ display: 'flex', justifyContent: 'center', width: '100%', marginBottom: '28px' }}>
                <div id="googleBtn" style={{ minHeight: '44px' }}></div>
              </div>

              {/* Leyenda de Seguridad */}
              <p style={{ 
                fontSize: '12px', 
                color: '#6e7582', 
                textAlign: 'center', 
                lineHeight: '1.6', 
                margin: 0,
                padding: '0 5px'
              }}>
                Para garantizar la seguridad de los expedientes clínicos, el acceso está restringido a correos institucionales o de Gmail verificados.
              </p>
            </>
          )}
        </div>
      </div>
    </AuthLayout>
  );
};