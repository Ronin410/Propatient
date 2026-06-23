import React, { useState, useEffect } from 'react';
import { useAuth } from '../context/AuthContext';
import api from '../api/axios';
import { useNavigate } from 'react-router-dom';
import './Login.scss'; 

export const Login = () => {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const { login } = useAuth();
  const navigate = useNavigate();

  // Reemplaza esto con el Client ID que te dio Google Cloud Console
  const GOOGLE_CLIENT_ID = "TU_CLIENT_ID_DE_GOOGLE.apps.googleusercontent.com";

  useEffect(() => {
    /* global google */
    // Inicializar el componente de Google una vez que el script se cargue
    if (typeof google !== 'undefined') {
      google.accounts.id.initialize({
        client_id: "744896665247-hrs7pmi72q2pog5dn9h1fh5n4nkpv18r.apps.googleusercontent.com"
,
        callback: handleGoogleResponse, // La función que recibirá el Token de Google
      });

      // Renderizar el botón oficial con estilos nativos en el contenedor con id="googleBtn"
      google.accounts.id.renderButton(
        document.getElementById("googleBtn"),
        { 
          theme: "outline", 
          size: "large", 
          text: "signin_with",
          width: "100%" 
        }
      );
    } else {
      console.error("El script de Google Identity Services no se ha cargado en index.html");
    }
  }, []);

  // Función encargada de recibir el ID Token (JWT) de Google y mandarlo a tu backend en Go
  const handleGoogleResponse = async (response: any) => {
    setIsLoading(true);
    setError(null);

    try {
      // Enviamos el token de Google al backend para su validación oficial
      const res = await api.post('/auth/google-login', { 
        idToken: response.credential 
      });
      
      // Guardamos el token propio de tu plataforma en el contexto de autenticación
      //login(res.data.token);

      // EVALUACIÓN DE ONBOARDING:
      // Tu backend en Go debe retornar banderas sobre el estado del usuario médico
      const { perfilCompletado, cedulaValidada } = res.data.userStatus;

      login(res.data.token, {
        profileCompleted: perfilCompletado,
        cedulaValidated: cedulaValidada
      });

      if (!perfilCompletado) {
        navigate('/registro/perfil');
      }else if (cedulaValidada === 'PENDIENTE') {
        navigate('/registro/validar-cedula');
      } else if (cedulaValidada === 'CAPTURADA') {
        // 🚀 SI YA SUBIÓ LOS DOCUMENTOS PERO NO SE HAN VALIDADO MANUALMENTE:
        alert("Tu cuenta y cédula profesional están siendo validadas por nuestro equipo técnico. Te notificaremos por correo una vez concluido el proceso.");
        //logout(); // Lo deslogueamos inmediatamente para que no pueda entrar usando rutas directas
        navigate('/login');
      } else if (cedulaValidada === 'VALIDADA') {
        navigate('/inicio'); // Acceso total únicamente si está VALIDADA
      }

    } catch (err: any) {
      setError(err.response?.data?.error || 'Error al autenticar con Google. Intente de nuevo.');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="login-wrapper">
      <div className="card login-card">
        <div className="logo-container">
            <div className="medical-logo" style={{ background: 'var(--color-primary)', borderRadius: '50%', margin: '0 auto 10px' }}></div>
            <p className="office-name">ProPatient Medical System</p>
        </div>

        <h1 className="access-title">Acceso al Sistema</h1>

        <div className="login-form">
          {error && (
            <div className="alert alert-danger" style={{ marginBottom: '1rem', padding: '10px', borderRadius: '4px', backgroundColor: '#fdd', color: '#a00', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span>⚠️</span>
              {error}
            </div>
          )}

          {isLoading ? (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', padding: '20px 0', gap: '10px' }}>
              <div className="spinner-border text-primary" role="status"></div>
              <p style={{ fontSize: '14px', color: '#666' }}>Validando credenciales...</p>
            </div>
          ) : (
            <>
              {/* CONTENEDOR DONDE GOOGLE RENDERIZARÁ SU BOTÓN OFICIAL */}
              <div id="googleBtn" style={{ width: '100%', marginBottom: '1.5rem' }}></div>

              <p style={{ fontSize: '12px', color: '#888', textAlign: 'center', lineHeight: '1.4' }}>
                Para garantizar la seguridad de los expedientes clínicos, el acceso está restringido a correos institucionales o de Gmail verificados.
              </p>
            </>
          )}

          {/* NOTA: El formulario anterior queda deshabilitado ya que solicitas acceso exclusivo por Gmail. 
              Si deseas mantener ambos métodos en paralelo para pruebas, puedes descomentar este bloque:
          
          <div className="separator" style={{ display: 'flex', alignItems: 'center', margin: '15px 0', color: '#aaa', fontSize: '12px' }}>
            <div style={{ flex: 1, height: '1px', background: '#eee' }}></div>
            <span style={{ padding: '0 10px' }}>O ENTRAR CON USUARIO</span>
            <div style={{ flex: 1, height: '1px', background: '#eee' }}></div>
          </div>

          <form onSubmit={handleSubmitOld}>
             ... Campos anteriores ...
          </form> 
          */}
        </div>
      </div>
    </div>
  );
};