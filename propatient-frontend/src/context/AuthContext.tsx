import React, { createContext, useContext, useState } from 'react';

// 1. Definimos la estructura del estatus del médico
interface UserStatus {
  profileCompleted: boolean;
  cedulaValidated: 'PENDIENTE' | 'CAPTURADA' | 'VALIDADA';
}

// 2. Agregamos el estatus y el tipado correcto al Contexto global
interface AuthContextType {
  token: string | null;
  userStatus: UserStatus | null; // <--- Agregado
  login: (token: string, status: UserStatus) => void; // <--- Modificado para recibir el estatus inicial
  logout: () => void;
  isAuthenticated: boolean;
  updateUserStatus: (status: Partial<UserStatus>) => void; // <--- Utilidad para cuando complete un paso
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [token, setToken] = useState<string | null>(localStorage.getItem('auth_token'));
  
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
    setToken(newToken);
    setUserStatus(status);
  };

  const logout = () => {
    localStorage.removeItem('auth_token');
    localStorage.removeItem('user_status');
    setToken(null);
    setUserStatus(null);
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
        login, 
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