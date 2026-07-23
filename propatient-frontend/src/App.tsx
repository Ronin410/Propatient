import { Suspense, lazy } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { OnboardingGuard } from './context/OnboardingGuard';
import { DoctorOnlyRoute } from './components/DoctorOnlyRoute';

// Todas las pantallas se cargan con lazy() en vez de importarse directo:
// sin esto, Vite empaqueta TODO en un solo archivo JS de ~2.4MB (incluye
// librerías pesadas como Leaflet y pdfmake, usadas solo en un puñado de
// pantallas), y cualquier visitante descarga ese archivo completo con solo
// abrir la landing pública. Con lazy(), cada pantalla es su propio
// pedacito que el navegador solo pide cuando el usuario navega ahí.
//
// React.lazy() exige que el módulo tenga un export default; como todas las
// pantallas de este proyecto usan export nombrado (`export const X`), se
// resuelve el named export al `default` que lazy() espera.

// Páginas de Estructura y Login
const Login = lazy(() => import('./pages/Login').then((m) => ({ default: m.Login })));
const StaffLogin = lazy(() => import('./pages/StaffLogin').then((m) => ({ default: m.StaffLogin })));
const AcceptStaffInvite = lazy(() => import('./pages/AcceptStaffInvite').then((m) => ({ default: m.AcceptStaffInvite })));
const ForgotStaffPassword = lazy(() => import('./pages/ForgotStaffPassword').then((m) => ({ default: m.ForgotStaffPassword })));
const ResetStaffPassword = lazy(() => import('./pages/ResetStaffPassword').then((m) => ({ default: m.ResetStaffPassword })));
const DashboardLayout = lazy(() => import('./pages/DashboardLayout').then((m) => ({ default: m.DashboardLayout })));

// Landing pública, directorio de doctores y agendamiento sin cuenta
const Landing = lazy(() => import('./pages/Landing').then((m) => ({ default: m.Landing })));
const DoctorDirectory = lazy(() => import('./pages/DoctorDirectory').then((m) => ({ default: m.DoctorDirectory })));
const PublicDoctorProfile = lazy(() => import('./pages/PublicDoctorProfile').then((m) => ({ default: m.PublicDoctorProfile })));
const PrivacyPolicy = lazy(() => import('./pages/PrivacyPolicy').then((m) => ({ default: m.PrivacyPolicy })));
const PrivacyPolicyContent = lazy(() => import('./pages/PrivacyPolicyContent').then((m) => ({ default: m.PrivacyPolicyContent })));
const TermsOfService = lazy(() => import('./pages/TermsOfService').then((m) => ({ default: m.TermsOfService })));
const TermsOfServiceContent = lazy(() => import('./pages/TermsOfServiceContent').then((m) => ({ default: m.TermsOfServiceContent })));

// Panel interno de administración (revisión de cédula profesional): sesión
// totalmente separada de la del doctor (ver AdminProtectedRoute).
const AdminLogin = lazy(() => import('./pages/AdminLogin').then((m) => ({ default: m.AdminLogin })));
const AdminPendingDoctors = lazy(() => import('./pages/AdminPendingDoctors').then((m) => ({ default: m.AdminPendingDoctors })));
const AdminDoctors = lazy(() => import('./pages/AdminDoctors').then((m) => ({ default: m.AdminDoctors })));
import { AdminProtectedRoute } from './components/AdminProtectedRoute';

// Nuevas Pantallas que estamos migrando
const AppointmentTracking = lazy(() => import('./pages/AppointmentTracking').then((m) => ({ default: m.AppointmentTracking })));
const PatientList = lazy(() => import('./pages/PatientList').then((m) => ({ default: m.PatientList })));
const PatientDetail = lazy(() => import('./pages/PatientDetail').then((m) => ({ default: m.PatientDetail })));
const PatientForm = lazy(() => import('./pages/PatientForm').then((m) => ({ default: m.PatientForm })));
const AppointmentCalendar = lazy(() => import('./pages/AppointmentCalendar').then((m) => ({ default: m.AppointmentCalendar })));
const AppointmentForm = lazy(() => import('./pages/AppointmentForm').then((m) => ({ default: m.AppointmentForm })));
const ConsultationManager = lazy(() => import('./pages/ConsultationManager').then((m) => ({ default: m.ConsultationManager })));
const AcceptTerms = lazy(() => import('./pages/AcceptTerms').then((m) => ({ default: m.AcceptTerms })));
const CompleteProfile = lazy(() => import('./pages/CompleteProfile').then((m) => ({ default: m.CompleteProfile })));
const ValidateLicense = lazy(() => import('./pages/ValidateLicense').then((m) => ({ default: m.ValidateLicense })));
const DoctorProfile = lazy(() => import('./pages/DoctorProfile').then((m) => ({ default: m.DoctorProfile })));
const SettingsNotes = lazy(() => import('./pages/SettingsNotes').then((m) => ({ default: m.SettingsNotes })));
const StaffManagement = lazy(() => import('./pages/StaffManagement').then((m) => ({ default: m.StaffManagement })));
const BillingPage = lazy(() => import('./pages/BillingPage').then((m) => ({ default: m.BillingPage })));
const ClinicManagement = lazy(() => import('./pages/ClinicManagement').then((m) => ({ default: m.ClinicManagement })));
const AcceptClinicInvite = lazy(() => import('./pages/AcceptClinicInvite').then((m) => ({ default: m.AcceptClinicInvite })));
const WorkingHours = lazy(() => import('./pages/WorkingHours').then((m) => ({ default: m.WorkingHours })));
const Reviews = lazy(() => import('./pages/Reviews').then((m) => ({ default: m.Reviews })));
const SubmitReview = lazy(() => import('./pages/SubmitReview').then((m) => ({ default: m.SubmitReview })));
const PublicUpload = lazy(() => import('./pages/PublicUpload').then((m) => ({ default: m.PublicUpload })));

// Componente para proteger rutas privadas básicas
const ProtectedRoute = ({ children }: { children: React.ReactNode }) => {
  const { isAuthenticated } = useAuth();

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
};

// Pantalla mínima mientras se descarga el código de la pantalla destino —
// solo se ve una fracción de segundo en conexiones normales, tronaría verla
// mucho tiempo solo en redes muy lentas.
const RouteLoading = () => (
  <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '60vh', color: 'var(--color-primary)' }}>
    <div className="spinner-border" role="status"></div>
  </div>
);

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
        <Suspense fallback={<RouteLoading />}>
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
            <Route path="/resena/:token" element={<SubmitReview />} />
            <Route path="/public-upload/:token" element={<PublicUpload />} />

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
            <Route
              path="/admin/doctores"
              element={
                <AdminProtectedRoute>
                  <AdminDoctors />
                </AdminProtectedRoute>
              }
            />

            {/* 📑 SECCIÓN ONBOARDING: Totalmente independiente de la estructura del Dashboard */}
            <Route
              path="/registro/terminos"
              element={
                <ProtectedRoute>
                  <AcceptTerms />
                </ProtectedRoute>
              }
            />
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
            {/* Aceptar invitación a clínica: a diferencia de la de personal,
                requiere sesión iniciada (el invitado ya tiene cuenta propia) */}
            <Route
              path="/clinica/invitacion/:token"
              element={
                <ProtectedRoute>
                  <AcceptClinicInvite />
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
                {/* Accesible también para personal: el horario laboral es una
                    herramienta operativa del día a día, no un dato exclusivo
                    del doctor (ver internal/handlers/schedule_handler.go). */}
                <Route path="horario" element={<WorkingHours />} />
                {/* Historial clínico, contenido de consultas y configuración del
                    doctor: el backend ya las bloquea para personal, aquí solo
                    evitamos que lleguen a una pantalla que va a fallar. */}
                <Route path="pacientes/:id" element={<DoctorOnlyRoute><PatientDetail /></DoctorOnlyRoute>} />
                <Route path="consulta/:appointmentId" element={<DoctorOnlyRoute><ConsultationManager /></DoctorOnlyRoute>} />
                <Route path="profile" element={<DoctorOnlyRoute><DoctorProfile /></DoctorOnlyRoute>} />
                <Route path="ajustes-notas" element={<DoctorOnlyRoute><SettingsNotes /></DoctorOnlyRoute>} />
                <Route path="personal" element={<DoctorOnlyRoute><StaffManagement /></DoctorOnlyRoute>} />
                <Route path="resenas" element={<DoctorOnlyRoute><Reviews /></DoctorOnlyRoute>} />
                {/* Accesible también para personal: si el consultorio se queda
                    sin prueba/suscripción, ambos roles terminan aquí (ver el
                    interceptor de 402 en src/api/axios.ts). */}
                <Route path="billing" element={<BillingPage />} />
                {/* Igual que "billing": accesible también para personal (el
                    propio componente muestra un aviso si isStaff). */}
                <Route path="clinica" element={<ClinicManagement />} />
              </Route>
            </Route>

            {/* Fallback general */}
            <Route path="*" element={<Navigate to="/inicio" replace />} />
          </Routes>
        </Suspense>
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;
