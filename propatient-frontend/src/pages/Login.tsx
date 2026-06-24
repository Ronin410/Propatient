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

  // Separamos la función de renderizado para poder llamarla en cualquier momento
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
          // 🚀 CLAVE: Limpiamos por completo cualquier residuo HTML previo antes de inyectar
          container.innerHTML = ""; 
          
          google.accounts.id.renderButton(
            container,
            { 
              theme: "outline", 
              size: "large", 
              text: "signin_with",
              width: "100%" 
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
    // Primera carga síncrona/asíncrona segura
    renderGoogleButton();
    const timer = setTimeout(renderGoogleButton, 150);

    return () => clearTimeout(timer);
  }, [isLoading]); // Re-renderiza y prepara el botón cada vez que termina un estado de carga

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

      // Evaluamos el estado antes de mutar la sesión global del contexto
      if (!perfilCompletado) {
        login(res.data.token, { profileCompleted: perfilCompletado, cedulaValidated: cedulaValidada });
        navigate('/registro/perfil');
      } else if (cedulaValidada === 'PENDIENTE') {
        login(res.data.token, { profileCompleted: perfilCompletado, cedulaValidated: cedulaValidada });
        navigate('/registro/validar-cedula');
      } else if (cedulaValidada === 'CAPTURADA') {
        // 🎯 AQUÍ ESTÁ EL CAMBIO:
        // Mostramos la alerta informativa al médico.
        alert("Tu cuenta y cédula profesional están siendo validadas por nuestro equipo técnico. Te notificaremos por correo una vez concluido el proceso.");
        
        // Apagamos el loader del login. Al pasar a false, el useEffect se dispara solo, 
        // limpia el div#googleBtn y vuelve a pintar el botón nativo listo para usarse de nuevo.
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
      <div className="card" style={{ padding: '35px', backgroundColor: '#ffffff', borderRadius: '12px', boxShadow: 'var(--shadow)' }}>
        <div style={{ textAlign: 'center', marginBottom: '24px' }}>
          <div style={{
            width: '48px',
            height: '48px',
            background: 'var(--accent, #aa3bff)',
            borderRadius: '50%',
            margin: '0 auto 12px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#fff',
            fontWeight: 'bold',
            fontSize: '20px'
          }}>
            +
          </div>
          <h1 style={{ fontSize: '24px', fontWeight: 700, color: 'var(--text-h)', margin: 0 }}>Acceso al Sistema</h1>
          <p style={{ fontSize: '14px', color: 'var(--text)', marginTop: '4px' }}>ProPatient Medical System</p>
        </div>

        <div className="login-form">
          {error && (
            <div className="alert alert-danger" style={{ marginBottom: '1.5rem', padding: '12px', borderRadius: '6px', backgroundColor: '#fee2e2', color: '#b91c1c', display: 'flex', alignItems: 'center', gap: '8px', fontSize: '14px' }}>
              <span>⚠️</span>
              {error}
            </div>
          )}

          {isLoading ? (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', padding: '30px 0', gap: '12px' }}>
              <div className="spinner-border text-primary" role="status"></div>
              <p style={{ fontSize: '14px', color: 'var(--text)' }}>Validando identidad médica...</p>
            </div>
          ) : (
            <>
              {/* Contenedor reseteable por el renderGoogleButton */}
              <div id="googleBtn" style={{ width: '100%', minHeight: '44px', marginBottom: '1.5rem' }}></div>

              <p style={{ fontSize: '12px', color: 'var(--text)', textAlign: 'center', lineHeight: '1.5', margin: 0 }}>
                Para garantizar la seguridad de los expedientes clínicos, el acceso está restringido a correos institucionales o de Gmail verificados.
              </p>
            </>
          )}
        </div>
      </div>
    </AuthLayout>
  );
};