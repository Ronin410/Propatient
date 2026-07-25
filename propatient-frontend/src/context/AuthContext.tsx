import React, { createContext, useContext, useEffect, useState } from 'react';

// 1. Definimos la estructura del estatus del médico
interface UserStatus {
  profileCompleted: boolean;
  cedulaValidated: 'PENDIENTE' | 'CAPTURADA' | 'VALIDADA';
  // Aceptación obligatoria de Términos y Condiciones + Aviso de Privacidad
  // (ver AcceptTerms.tsx / OnboardingGuard.tsx). Opcional en el tipo para
  // no romper sesiones ya guardadas en localStorage de antes de este
  // campo — se trata como `false` si no viene definido.
  termsAccepted?: boolean;
}

type Role = 'MEDICO' | 'STAFF';

// 2. Agregamos el estatus y el tipado correcto al Contexto global
interface AuthContextType {
  token: string | null;
  userStatus: UserStatus | null; // <--- Agregado
  role: Role;
  isStaff: boolean;
  // Nombre a mostrar en el sidebar: el propio del doctor, o el del
  // consultorio para una sesión de personal. Vive en el contexto (no solo
  // en localStorage) para que cualquier pantalla que lo actualice (Perfil,
  // onboarding) se refleje de inmediato en el layout sin recargar la
  // página ni volver a iniciar sesión.
  doctorName: string | null;
  setDoctorName: (name: string) => void;
  // fullName es opcional porque no todas las respuestas de login lo traen
  // (ej. un registro manual viejo); sin él, el sidebar usa un genérico en
  // vez de quedarse con un nombre de otra sesión.
  login: (token: string, status: UserStatus, fullName?: string) => void; // <--- Modificado para recibir el estatus inicial
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
  const [doctorName, setDoctorNameState] = useState<string | null>(() => localStorage.getItem('doctor_name'));

  // Setter expuesto al resto de la app (Perfil, onboarding): guarda en
  // localStorage para que sobreviva un F5, y actualiza el estado para que
  // el sidebar se refresque de inmediato en la misma sesión.
  const setDoctorName = (name: string) => {
    localStorage.setItem('doctor_name', name);
    setDoctorNameState(name);
  };

  // Al iniciar sesión, guardamos tanto el Token como las banderas que mandó Go
  const login = (newToken: string, status: UserStatus, fullName?: string) => {
    localStorage.setItem('auth_token', newToken);
    localStorage.setItem('user_status', JSON.stringify(status));
    localStorage.setItem('user_role', 'MEDICO');
    if (fullName) {
      setDoctorName(fullName);
    } else {
      localStorage.removeItem('doctor_name');
      setDoctorNameState(null);
    }
    setToken(newToken);
    setUserStatus(status);
    setRole('MEDICO');
  };

  const loginStaff = (newToken: string, doctorName: string) => {
    localStorage.setItem('auth_token', newToken);
    localStorage.setItem('user_role', 'STAFF');
    localStorage.removeItem('user_status');
    setDoctorName(doctorName);
    setToken(newToken);
    setUserStatus(null);
    setRole('STAFF');
  };

  const logout = () => {
    localStorage.removeItem('auth_token');
    localStorage.removeItem('user_status');
    localStorage.removeItem('user_role');
    localStorage.removeItem('doctor_name');
    setToken(null);
    setUserStatus(null);
    setDoctorNameState(null);
    setRole('MEDICO');
  };

  // "auth_token" vive en localStorage, compartido por TODAS las pestañas
  // del mismo origen — si en la pestaña B inicias sesión con otro doctor
  // (o cierras sesión), la pestaña A queda con el token viejo en memoria
  // pero cada petición nueva de axios ya manda el token nuevo (lo lee de
  // localStorage en cada request, ver api/axios.ts), así que A terminaría
  // mostrando datos de B sin avisar. El evento "storage" del navegador
  // solo se dispara en las OTRAS pestañas (nunca en la que hizo el
  // cambio), así que es el lugar correcto para detectar esto y recargar
  // en vez de seguir operando con una sesión que ya no es la vigente.
  useEffect(() => {
    const handleStorageChange = (event: StorageEvent) => {
      if (event.key !== 'auth_token' || event.newValue === event.oldValue) return;
      window.alert('Tu sesión cambió en otra pestaña (inicio o cierre de sesión). Esta pestaña se va a recargar.');
      window.location.reload();
    };
    window.addEventListener('storage', handleStorageChange);
    return () => window.removeEventListener('storage', handleStorageChange);
  }, []);

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
        doctorName,
        setDoctorName,
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
