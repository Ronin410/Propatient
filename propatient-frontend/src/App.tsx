import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { OnboardingGuard } from './context/OnboardingGuard';

// Páginas de Estructura y Login
import { Login } from './pages/Login';
import { DashboardLayout } from './pages/DashboardLayout';

// Nuevas Pantallas que estamos migrando
import { AppointmentTracking } from './pages/AppointmentTracking';
import { PatientList } from './pages/PatientList';
import { PatientDetail } from './pages/PatientDetail';
import { PatientForm } from './pages/PatientForm';
import { AppointmentCalendar } from './pages/AppointmentCalendar';
import { AppointmentForm } from './pages/AppointmentForm';
import { ConsultationManager } from './pages/ConsultationManager';
import { CompleteProfile } from './pages/CompleteProfile';
import { ValidateLicense } from './pages/ValidateLicense';

// Componente para proteger rutas privadas básicas
const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const { isAuthenticated } = useAuth();
  
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
};

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          {/* 🔓 RUTA PÚBLICA */}
          <Route path="/login" element={<Login />} />

          {/* 📑 SECCIÓN ONBOARDING: Totalmente independiente de la estructura del Dashboard */}
          <Route
            path="/registro/perfil"
            element={
              <ProtectedRoute>
                <CompleteProfile />
              </ProtectedRoute>
            }
          />
          <Route
            path="/registro/validar-cedula"
            element={
              <ProtectedRoute>
                <ValidateLicense />
              </ProtectedRoute>
            }
          />

          {/* 💻 SISTEMA MÉDICO PRINCIPAL: Envuelto por el OnboardingGuard */}
          <Route element={<OnboardingGuard />}>
            <Route
              element={
                <ProtectedRoute>
                  <DashboardLayout />
                </ProtectedRoute>
              }
            >
              {/* Rutas Hijas que sí llevan el menú lateral y barra de navegación */}
              <Route index element={<Navigate to="/inicio" replace />} />
              <Route path="inicio" element={<AppointmentTracking />} />
              <Route path="pacientes" element={<PatientList />} />
              <Route path="pacientes/:id" element={<PatientDetail />} />
              <Route path="pacientes/nuevo" element={<PatientForm />} />
              <Route path="pacientes/editar/:id" element={<PatientForm />} />
              <Route path="calendar" element={<AppointmentCalendar />} />
              <Route path="appointments/new" element={<AppointmentForm />} />
              <Route path="consulta/:appointmentId" element={<ConsultationManager />} />
              <Route path="profile" element={<div>Perfil del Doctor</div>} />
            </Route>
          </Route>

          {/* Fallback general */}
          <Route path="*" element={<Navigate to="/inicio" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;