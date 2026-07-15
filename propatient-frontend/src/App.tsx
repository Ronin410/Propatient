import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { OnboardingGuard } from './context/OnboardingGuard';
import { DoctorOnlyRoute } from './components/DoctorOnlyRoute';

// Páginas de Estructura y Login
import { Login } from './pages/Login';
import { StaffLogin } from './pages/StaffLogin';
import { AcceptStaffInvite } from './pages/AcceptStaffInvite';
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
import { DoctorProfile } from './pages/DoctorProfile';
import { SettingsNotes } from './pages/SettingsNotes';
import { StaffManagement } from './pages/StaffManagement';
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
          {/* 🔓 RUTAS PÚBLICAS */}
          <Route path="/login" element={<Login />} />
          <Route path="/staff-login" element={<StaffLogin />} />
          <Route path="/personal/invitacion/:token" element={<AcceptStaffInvite />} />

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
              <Route path="pacientes/nuevo" element={<PatientForm />} />
              <Route path="pacientes/editar/:id" element={<PatientForm />} />
              <Route path="calendar" element={<AppointmentCalendar />} />
              <Route path="appointments/new" element={<AppointmentForm />} />
              {/* Historial clínico, contenido de consultas y configuración del
                  doctor: el backend ya las bloquea para personal, aquí solo
                  evitamos que lleguen a una pantalla que va a fallar. */}
              <Route path="pacientes/:id" element={<DoctorOnlyRoute><PatientDetail /></DoctorOnlyRoute>} />
              <Route path="consulta/:appointmentId" element={<DoctorOnlyRoute><ConsultationManager /></DoctorOnlyRoute>} />
              <Route path="profile" element={<DoctorOnlyRoute><DoctorProfile /></DoctorOnlyRoute>} />
              <Route path="ajustes-notas" element={<DoctorOnlyRoute><SettingsNotes /></DoctorOnlyRoute>} />
              <Route path="personal" element={<DoctorOnlyRoute><StaffManagement /></DoctorOnlyRoute>} />
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