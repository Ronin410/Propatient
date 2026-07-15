import React, { createContext, useContext, useState } from 'react';

// 1. Definimos la estructura del estatus del médico
interface UserStatus {
  profileCompleted: boolean;
  cedulaValidated: 'PENDIENTE' | 'CAPTURADA' | 'VALIDADA';
}

type Role = 'MEDICO' | 'STAFF';

// 2. Agregamos el estatus y el tipado correcto al Contexto global
interface AuthContextType {
  token: string | null;
  userStatus: UserStatus | null; // <--- Agregado
  role: Role;
  isStaff: boolean;
  login: (token: string, status: UserStatus) => void; // <--- Modificado para recibir el estatus inicial
  // Sesión de personal: sin onboarding, con el nombre del consultorio del
  // doctor (no hay UserStatus, el personal nunca pasa por ese flujo).
  loginStaff: (token: string, doctorName: string) => void;
  logout: () => void;
  isAuthenticated: boolean;
  updateUserStatus: (status: Partial<UserStatus>) => void; // <--- Utilidad para cuando complete un paso
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

function getStoredRole(): Role {
  return localStorage.getItem('user_role') === 'STAFF' ? 'STAFF' : 'MEDICO';
}

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [token, setToken] = useState<string | null>(localStorage.getItem('auth_token'));
  const [role, setRole] = useState<Role>(getStoredRole);

  // ✅ El Hook de estado DEBE ir aquí adentro.
  // Intentamos recuperar el estatus guardado para evitar que al dar F5 lo mande al inicio del registro.
  const [userStatus, setUserStatus] = useState<UserStatus | null>(() => {
    const savedStatus = localStorage.getItem('user_status');
    return savedStatus ? JSON.parse(savedStatus) : null;
  });

  // Al iniciar sesión, guardamos tanto el Token como las banderas que mandó Go
  const login = (newToken: string, status: UserStatus) => {
    localStorage.setItem('auth_token', newToken);
    localStorage.setItem('user_status', JSON.stringify(status));
    localStorage.setItem('user_role', 'MEDICO');
    setToken(newToken);
    setUserStatus(status);
    setRole('MEDICO');
  };

  const loginStaff = (newToken: string, doctorName: string) => {
    localStorage.setItem('auth_token', newToken);
    localStorage.setItem('user_role', 'STAFF');
    localStorage.setItem('doctor_name', doctorName);
    localStorage.removeItem('user_status');
    setToken(newToken);
    setUserStatus(null);
    setRole('STAFF');
  };

  const logout = () => {
    localStorage.removeItem('auth_token');
    localStorage.removeItem('user_status');
    localStorage.removeItem('user_role');
    setToken(null);
    setUserStatus(null);
    setRole('MEDICO');
  };

  // Función útil para cuando el doctor complete el perfil o valide su cédula
  const updateUserStatus = (newStatus: Partial<UserStatus>) => {
    setUserStatus((prev) => {
      if (!prev) return null;
      const updated = { ...prev, ...newStatus };
      localStorage.setItem('user_status', JSON.stringify(updated));
      return updated;
    });
  };

  return (
    <AuthContext.Provider
      value={{
        token,
        userStatus,
        role,
        isStaff: role === 'STAFF',
        login,
        loginStaff,
        logout,
        isAuthenticated: !!token,
        updateUserStatus
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth debe usarse dentro de AuthProvider');
  return context;
};
