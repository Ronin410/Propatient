import React from 'react';
import { Navigate } from 'react-router-dom';

// Guarda de las rutas de /admin: deliberadamente NO usa AuthContext (esa es
// la sesión del doctor). Aquí solo importa si hay un "admin_token" — la
// validez real del token la sigue verificando el backend en cada petición
// (ver AuthorizeSuperAdminJWT), esto solo evita mostrar la pantalla vacía
// un instante antes de que el interceptor de adminAxios redirija por un 401.
export const AdminProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const hasAdminToken = !!localStorage.getItem('admin_token');
  if (!hasAdminToken) {
    return <Navigate to="/admin/login" replace />;
  }
  return <>{children}</>;
};
