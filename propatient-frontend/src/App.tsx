import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { OnboardingGuard } from './context/OnboardingGuard';
// Páginas de Estructura y Login
import { Login } from './pages/Login';
import { DashboardLayout } from './pages/DashboardLayout';

// Nuevas Pantallas que estamos migrando
import { AppointmentTracking } from './pages/AppointmentTracking'; // La que creamos recién
import { PatientList } from './pages/PatientList';
import { PatientDetail } from './pages/PatientDetail';
import { PatientForm } from './pages/PatientForm';
import { AppointmentCalendar } from './pages/AppointmentCalendar';
import { AppointmentForm } from './pages/AppointmentForm';
import { ConsultationManager } from './pages/ConsultationManager';
import { CompleteProfile } from './pages/CompleteProfile';
import { ValidateLicense } from './pages/ValidateLicense';

// Componente para proteger rutas privadas
const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const { isAuthenticated } = useAuth();
  
  // Si no está autenticado, lo manda al login
  if (!isAuthenticated) {
    return <Navigate to="/login" />;
  }

  return <>{children}</>;
};

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          {/* Ruta Pública */}
          <Route path="/login" element={<Login />} />
          
{/* ========================================================= */}
          {/* 2. PANTALLAS DE REGISTRO AISLADAS (Sin menús laterales)    */}
          {/* ========================================================= */}
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


<Route element={<OnboardingGuard />}>
          {/* Rutas Privadas (Dentro del Dashboard) */}
          <Route 
            path="/" 
            element={
              <ProtectedRoute>
                <DashboardLayout />
              </ProtectedRoute>
            }
          >
            {/* Al entrar a "/" redirige automáticamente a "/inicio" */}
            <Route index element={<Navigate to="/inicio" replace />} />
            
            {/* Inicio: Seguimiento de Citas (Dashboard Home) */}
            <Route path="inicio" element={<AppointmentTracking />} />


            {/* Pacientes: Lista y gestión */}
            <Route path="pacientes" element={<PatientList />} />
            <Route path="pacientes/:id" element={<PatientDetail />} />
            <Route path="pacientes/nuevo" element={<PatientForm />} />
            <Route path="pacientes/editar/:id" element={<PatientForm />} />
            
            {/* Citas: El Calendario completo */}
            <Route path="calendar" element={<AppointmentCalendar />} />
            <Route path="appointments/new" element={<AppointmentForm />} />
            
            {/* Flujo de Atención Médica */}
            <Route path="consulta/:appointmentId" element={<ConsultationManager />} />
            
            {/* Perfil: Ajustes del doctor */}
            <Route path="profile" element={<div>Perfil del Doctor</div>} />
          </Route>
        </Route>

          {/* Manejo de rutas inexistentes: manda al login o al inicio según auth */}
          <Route path="*" element={<Navigate to="/inicio" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;