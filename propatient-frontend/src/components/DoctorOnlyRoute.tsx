import React from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

// Envuelve pantallas que el backend ya bloquea para cuentas de personal
// (historial clínico, contenido de consultas, perfil/configuración del
// doctor): evita que el personal llegue a una pantalla que solo va a
// mostrarle errores 403, mandándolo de vuelta al dashboard.
export const DoctorOnlyRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { isStaff } = useAuth();

  if (isStaff) {
    return <Navigate to="/inicio" replace />;
  }

  return <>{children}</>;
};
