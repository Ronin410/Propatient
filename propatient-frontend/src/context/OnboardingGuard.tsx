import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export const OnboardingGuard = () => {
  const { isAuthenticated, userStatus } = useAuth();

  // 1. Si ni siquiera está logueado, al login de cabeza
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  // Si aún está cargando el estado del usuario, puedes retornar un spinner breve
  if (!userStatus) return <div>Cargando datos de verificación...</div>;

  // 2. Si NO ha completado la información profesional, lo forzamos a quedarse ahí
  if (!userStatus.profileCompleted) {
    return <Navigate to="/registro/perfil" replace />;
  }

  // 3. Si completó el perfil pero NO ha validado la cédula, lo mandamos a la cédula
  if (userStatus.cedulaValidated !== 'VALIDADA') {
    return <Navigate to="/registro/validar-cedula" replace />;
  }

  // 4. Si ya cumplió con todo, se le permite renderizar las rutas del Dashboard (las hijas)
  return <Outlet />;
};