import { useEffect } from 'react';
import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import api from '../api/axios';

export const OnboardingGuard = () => {
  const { isAuthenticated, isStaff, userStatus, updateUserStatus } = useAuth();

  // Una sesión que ya estaba abierta cuando el aviso legal cambió de
  // versión no se entera hasta el siguiente login (login es la única otra
  // fuente de userStatus) — este chequeo, una sola vez por sesión montada,
  // cierra ese hueco: si el backend dice que la aceptación guardada ya no
  // corresponde a la versión vigente (ver models.CurrentLegalNoticeVersion
  // y GetCurrentDoctor "termsUpToDate"), se baja la bandera local para que
  // el guard de abajo redirija a /registro/terminos igual que si nunca
  // hubiera aceptado.
  useEffect(() => {
    if (!isAuthenticated || isStaff || !userStatus?.termsAccepted) return;
    api.get('/doctor/me')
      .then((res) => {
        if (res.data?.termsUpToDate === false) {
          updateUserStatus({ termsAccepted: false });
        }
      })
      .catch(() => {
        // Sin red o suscripción vencida: no hacemos nada, se revisará en
        // el próximo login o la próxima vez que haya conexión.
      });
  }, [isAuthenticated, isStaff, userStatus?.termsAccepted, updateUserStatus]);

  // 1. Si ni siquiera está logueado, al login de cabeza
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  // El personal no tiene cédula ni onboarding propio: siempre entra directo
  // al dashboard del consultorio del doctor que lo invitó.
  if (isStaff) {
    return <Outlet />;
  }

  // Si aún está cargando el estado del usuario, puedes retornar un spinner breve
  if (!userStatus) return <div>Cargando datos de verificación...</div>;

  // 2. Si NO ha aceptado los Términos y Condiciones + Aviso de Privacidad
  // (o los aceptó pero de una versión anterior, ver el efecto de arriba),
  // no puede pasar de ahí — es el primer paso, antes que el perfil.
  if (!userStatus.termsAccepted) {
    return <Navigate to="/registro/terminos" replace />;
  }

  // 3. Si NO ha completado la información profesional, lo forzamos a quedarse ahí
  if (!userStatus.profileCompleted) {
    return <Navigate to="/registro/perfil" replace />;
  }

  // 4. Si completó el perfil pero NO ha validado la cédula, lo mandamos a la cédula
  if (userStatus.cedulaValidated !== 'VALIDADA') {
    return <Navigate to="/registro/validar-cedula" replace />;
  }

  // 5. Si ya cumplió con todo, se le permite renderizar las rutas del Dashboard (las hijas)
  return <Outlet />;
};
