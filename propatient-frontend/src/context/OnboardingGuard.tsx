import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export const OnboardingGuard = () => {
  const { isAuthenticated, isStaff, userStatus } = useAuth();

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

  // 2. Si NO ha aceptado los Términos y Condiciones + Aviso de Privacidad,
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