import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { OnboardingGuard } from './context/OnboardingGuard';
import { DoctorOnlyRoute } from './components/DoctorOnlyRoute';

// Páginas de Estructura y Login
import { Login } from './pages/Login';
import { StaffLogin } from './pages/StaffLogin';
import { AcceptStaffInvite } from './pages/AcceptStaffInvite';
import { DashboardLayout } from './pages/DashboardLayout';

// Landing pública, directorio de doctores y agendamiento sin cuenta
import { Landing } from './pages/Landing';
import { DoctorDirectory } from './pages/DoctorDirectory';
import { PublicDoctorProfile } from './pages/PublicDoctorProfile';
import { PrivacyPolicy } from './pages/PrivacyPolicy';

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
import { BillingPage } from './pages/BillingPage';
// Componente para proteger rutas privadas básicas
const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const { isAuthenticated } = useAuth();

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
};

// "/" es pública (landing) para quien no tiene sesión; quien ya inició
// sesión pasa directo a su panel. No es un "index" del dashboard: por eso
// vive fuera de la sección protegida, como su propia ruta con path="/".
const RootRoute = () => {
  const { isAuthenticated } = useAuth();
  return isAuthenticated ? <Navigate to="/inicio" replace /> : <Landing />;
};

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          {/* 🔓 RUTAS PÚBLICAS */}
          <Route path="/" element={<RootRoute />} />
          <Route path="/doctores" element={<DoctorDirectory />} />
          <Route path="/dr/:slug" element={<PublicDoctorProfile />} />
          <Route path="/privacidad" element={<PrivacyPolicy />} />
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
              {/* Accesible también para personal: si el consultorio se queda
                  sin prueba/suscripción, ambos roles terminan aquí (ver el
                  interceptor de 402 en src/api/axios.ts). */}
              <Route path="billing" element={<BillingPage />} />
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