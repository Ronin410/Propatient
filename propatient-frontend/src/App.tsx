import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { OnboardingGuard } from './context/OnboardingGuard';
import { DoctorOnlyRoute } from './components/DoctorOnlyRoute';

// Páginas de Estructura y Login
import { Login } from './pages/Login';
import { StaffLogin } from './pages/StaffLogin';
import { AcceptStaffInvite } from './pages/AcceptStaffInvite';
import { ForgotStaffPassword } from './pages/ForgotStaffPassword';
import { ResetStaffPassword } from './pages/ResetStaffPassword';
import { DashboardLayout } from './pages/DashboardLayout';

// Landing pública, directorio de doctores y agendamiento sin cuenta
import { Landing } from './pages/Landing';
import { DoctorDirectory } from './pages/DoctorDirectory';
import { PublicDoctorProfile } from './pages/PublicDoctorProfile';
import { PrivacyPolicy } from './pages/PrivacyPolicy';
import { PrivacyPolicyContent } from './pages/PrivacyPolicyContent';
import { TermsOfService } from './pages/TermsOfService';
import { TermsOfServiceContent } from './pages/TermsOfServiceContent';

// Panel interno de administración (revisión de cédula profesional): sesión
// totalmente separada de la del doctor (ver AdminProtectedRoute).
import { AdminLogin } from './pages/AdminLogin';
import { AdminPendingDoctors } from './pages/AdminPendingDoctors';
import { AdminProtectedRoute } from './components/AdminProtectedRoute';

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

// "/" es la landing pública y siempre se muestra ahí, tenga o no sesión
// iniciada — así el logo/marca "ProPatient" siempre lleva al inicio
// público real, en vez de que el login se sienta como la pantalla
// principal del sitio. Con sesión iniciada, Landing muestra un botón
// "Ir a mi panel" en vez de "Iniciar sesión" para volver al dashboard.
const RootRoute = () => <Landing />;

// Con sesión iniciada, /privacidad y /terminos se muestran dentro del
// DashboardLayout (mismo menú lateral que el resto del panel) en vez de la
// página pública standalone, para que el doctor nunca "salga" de la app al
// consultarlas.
const PrivacyRoute = () => {
  const { isAuthenticated } = useAuth();
  if (isAuthenticated) {
    return (
      <DashboardLayout>
        <PrivacyPolicyContent />
      </DashboardLayout>
    );
  }
  return <PrivacyPolicy />;
};

const TermsRoute = () => {
  const { isAuthenticated } = useAuth();
  if (isAuthenticated) {
    return (
      <DashboardLayout>
        <TermsOfServiceContent />
      </DashboardLayout>
    );
  }
  return <TermsOfService />;
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
          <Route path="/privacidad" element={<PrivacyRoute />} />
          <Route path="/terminos" element={<TermsRoute />} />
          <Route path="/login" element={<Login />} />
          <Route path="/staff-login" element={<StaffLogin />} />
          <Route path="/personal/invitacion/:token" element={<AcceptStaffInvite />} />
          <Route path="/personal/recuperar" element={<ForgotStaffPassword />} />
          <Route path="/personal/restablecer/:token" element={<ResetStaffPassword />} />

          {/* 🔐 PANEL INTERNO DE ADMINISTRACIÓN: sesión separada de la del doctor */}
          <Route path="/admin/login" element={<AdminLogin />} />
          <Route
            path="/admin/pendientes"
            element={
              <AdminProtectedRoute>
                <AdminPendingDoctors />
              </AdminProtectedRoute>
            }
          />

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