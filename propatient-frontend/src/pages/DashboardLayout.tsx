import React, { useEffect, useState } from 'react';
import { Outlet, Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useTheme } from '../context/ThemeContext';
import { Footer } from '../components/Footer';
import { InstallPwaButton } from '../components/InstallPwaButton';
import { PwaInstallGuide } from '../components/PwaInstallGuide';
import api from '../api/axios';
import type { StaffDoctorOption } from '../types';
import logo from '../assets/logo.png';
import './DashboardLayout.scss';

interface DashboardLayoutProps {
  // Si se pasan children, se muestran en vez del <Outlet /> de la ruta
  // anidada — usado por App.tsx para reutilizar el layout (con menú
  // lateral) en páginas que no son rutas hijas del dashboard, como
  // /privacidad cuando hay sesión iniciada.
  children?: React.ReactNode;
}

// Mismo número que TWILIO_WHATSAPP_FROM en el backend (ver .env.example) —
// abre WhatsApp directo con un mensaje precargado en vez de un chat propio
// dentro de la página: la respuesta le llega al superadmin en la misma
// bandeja que las de los pacientes, solo que clasificada como
// "DOCTOR_SUPPORT" (ver handlers.classifyInboundWhatsApp).
const supportWhatsAppNumber = import.meta.env.VITE_SUPPORT_WHATSAPP_NUMBER as string | undefined;

export const DashboardLayout: React.FC<DashboardLayoutProps> = ({ children }) => {
  const { logout, isStaff, doctorName: sessionDoctorName, setDoctorName, loginStaff } = useAuth();
  const { resolvedTheme, toggleTheme } = useTheme();
  const location = useLocation();
  const navigate = useNavigate();
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  // Consultorios a los que esta cuenta de personal tiene acceso activo, para
  // el selector de "cambiar de consultorio" de abajo — solo se muestra si
  // hay más de uno (clínica con personal compartido, ver ListMyDoctors).
  const [staffDoctors, setStaffDoctors] = useState<StaffDoctorOption[]>([]);
  const [switchingDoctor, setSwitchingDoctor] = useState(false);

  useEffect(() => {
    if (!isStaff) return;
    api.get('/staff/my-doctors')
      .then((res) => setStaffDoctors(res.data?.doctors || []))
      .catch(() => {
        // Sin red o error: simplemente no aparece el selector, no es crítico.
      });
  }, [isStaff]);

  const handleSwitchDoctor = async (doctorId: number) => {
    if (!doctorId || switchingDoctor) return;
    setSwitchingDoctor(true);
    try {
      const res = await api.post('/staff/switch-doctor', { doctorId });
      loginStaff(res.data.token, res.data.doctorName || '');
      // Recarga completa (no solo navigate): evita que quede en memoria
      // caché de pacientes/citas del consultorio anterior, mismo patrón
      // que handleDeleteAccount en DoctorProfile.
      window.location.href = '/inicio';
    } catch {
      setSwitchingDoctor(false);
      window.alert('No se pudo cambiar de consultorio. Intenta de nuevo.');
    }
  };

  // Autocompletar el nombre si la sesión ya estaba abierta antes de que
  // login()/Perfil empezaran a guardarlo (o si por lo que sea nunca se
  // guardó) — sin este fallback, una sesión vieja se queda mostrando el
  // genérico "Doctor" para siempre, porque login() solo corre una vez, al
  // iniciar sesión. Solo aplica al doctor: el personal ya lo recibe
  // siempre en loginStaff() al iniciar sesión.
  useEffect(() => {
    if (isStaff || sessionDoctorName) return;
    api.get('/doctor/me')
      .then((res) => {
        if (res.data?.fullName) setDoctorName(res.data.fullName);
      })
      .catch(() => {
        // Sin permiso (personal, suscripción vencida) o sin red: se queda
        // con el genérico, no hay nada más que intentar aquí.
      });
  }, [isStaff, sessionDoctorName, setDoctorName]);

  // Si el doctor pertenece a una clínica, "Suscripción" no aplica — su
  // acceso depende de la suscripción de la clínica, no de una propia (ver
  // GetBillingStatus/CreateCheckoutSession, que ya lo rechazan del lado
  // del backend). Se consulta aparte del efecto de arriba porque ese se
  // salta la llamada por completo si sessionDoctorName ya está guardado.
  const [doctorClinicId, setDoctorClinicId] = useState<number | null>(null);
  useEffect(() => {
    if (isStaff) return;
    api.get('/doctor/me')
      .then((res) => setDoctorClinicId(res.data?.clinicId ?? null))
      .catch(() => {
        // Sin red o sin permiso: se asume que no hay clínica y se deja ver
        // "Suscripción" — mismo criterio de mejor esfuerzo que arriba.
      });
  }, [isStaff]);

  // Nombre dinámico del contexto (login/loginStaff/Perfil/el autocompletado
  // de arriba lo actualizan ahí, ver AuthContext) — usar el contexto en vez
  // de leer localStorage directo aquí es lo que hace que un cambio de
  // nombre en el Perfil se refleje de inmediato en el sidebar sin recargar
  // la página. El genérico de abajo solo se ve mientras carga o si de
  // plano no se pudo obtener — nunca debe mostrar el nombre de una persona
  // real como si fuera un valor por defecto del sistema.
  const doctorName = sessionDoctorName || 'Doctor';
  const initialLetter = doctorName.replace('Dr. ', '').charAt(0).toUpperCase();

  // Cierra el menú al navegar a otra pantalla (relevante en móvil/tablet).
  useEffect(() => {
    setIsSidebarOpen(false);
  }, [location.pathname]);

  // El personal solo gestiona agenda y pacientes: sin Perfil, Ajustes ni
  // gestión de Personal (todo eso ya está bloqueado también en el backend).
  // "Horario" es la excepción a propósito: tanto el doctor como su
  // personal necesitan poder configurarlo (ver schedule_handler.go, sin
  // RequireDoctorRole).
  const menuItems = [
    { label: 'Dashboard', icon: 'home', route: '/inicio' },
    { label: 'Pacientes', icon: 'people', route: '/pacientes' },
    { label: 'Citas', icon: 'calendar_month', route: '/calendar' },
    { label: 'Horario', icon: 'schedule', route: '/horario' },
    ...(!isStaff ? [
      { label: 'Personal', icon: 'badge', route: '/personal' },
      { label: 'Reseñas', icon: 'reviews', route: '/resenas' },
      // Oculto para un doctor de clínica: su suscripción la gestiona el
      // dueño desde "Mi Clínica", no aquí (ver doctorClinicId arriba).
      ...(doctorClinicId == null ? [{ label: 'Suscripción', icon: 'credit_card', route: '/billing' }] : []),
      { label: 'Mi Clínica', icon: 'apartment', route: '/clinica' },
      { label: 'Perfil', icon: 'settings', route: '/profile' },
      { label: 'Ajustes Notas', icon: 'tune', route: '/ajustes-notas' }
    ] : [])
  ];

  const handleLogout = () => {
    // 1. Preguntamos primero si realmente desea cerrar la sesión general
    const confirmLogout = window.confirm("¿Estás seguro de que deseas cerrar sesión? Cualquier consulta activa o cambio sin guardar se perderá.");
    
    if (confirmLogout) {
      // 2. Si acepta, limpiamos los bloqueos manuales del historial para que no choquen
      window.onbeforeunload = null;
      
      // 3. Ejecutamos el cierre de sesión y redirección originales
      logout();
      navigate('/login');
    }
    // Si da cancelar, no hace absolutamente nada y se queda en la pantalla actual
  };

  return (
    <div className="dashboard-container">
      {/* BARRA SUPERIOR MÓVIL (solo visible en pantallas angostas) */}
      <header className="mobile-topbar">
        <button
          className="hamburger-btn"
          onClick={() => setIsSidebarOpen(true)}
          aria-label="Abrir menú"
        >
          <span className="material-icons-outlined">menu</span>
        </button>
        <p className="office-name">PROPatient</p>
      </header>

      {/* FONDO OSCURO AL ABRIR EL MENÚ EN MÓVIL/TABLET */}
      {isSidebarOpen && (
        <div className="sidebar-overlay" onClick={() => setIsSidebarOpen(false)} />
      )}

      {/* BARRA LATERAL (SIDEBAR) */}
      <aside className={`sidebar ${isSidebarOpen ? 'open' : ''}`}>
        <div>
          <div className="sidebar-header">
            <div className="logo-container">
              <img src={logo} alt="ProPatient" className="medical-logo" />
              <p className="office-name">PROPatient</p>
            </div>
            <button
              className="close-sidebar-btn"
              onClick={() => setIsSidebarOpen(false)}
              aria-label="Cerrar menú"
            >
              <span className="material-icons-outlined">close</span>
            </button>
          </div>

          <nav className="sidebar-nav">
            {menuItems.map((item) => (
              <Link
                key={item.label}
                to={item.route}
                className={`nav-item ${location.pathname === item.route ? 'active' : ''}`}
              >
                <span className="material-icons-outlined">{item.icon}</span>
                {item.label}
              </Link>
            ))}
          </nav>
        </div>

        {/* FOOTER DEL MENU CON IDENTIDAD */}
        <div className="sidebar-footer">
          <div className="doctor-profile-summary">
            <div className="avatar-mini">
              {initialLetter}
            </div>
            <p className="doctor-name">{doctorName}</p>
          </div>

          {isStaff && staffDoctors.length > 1 && (
            <div className="doctor-switcher">
              <select
                value=""
                disabled={switchingDoctor}
                aria-label="Cambiar de consultorio"
                onChange={(e) => handleSwitchDoctor(Number(e.target.value))}
              >
                <option value="">{switchingDoctor ? 'Cambiando de consultorio...' : 'Cambiar de consultorio...'}</option>
                {staffDoctors.map((doc) => (
                  <option key={doc.doctorId} value={doc.doctorId}>{doc.doctorName}</option>
                ))}
              </select>
            </div>
          )}

          <InstallPwaButton className="sidebar-install-btn" />
          {supportWhatsAppNumber && (
            <a
              className="theme-toggle-link"
              href={`https://wa.me/${supportWhatsAppNumber.replace(/[^0-9]/g, '')}?text=${encodeURIComponent('Hola, necesito ayuda con ProPatient.')}`}
              target="_blank"
              rel="noreferrer"
            >
              <span className="material-icons-outlined">support_agent</span>
              ¿Necesitas ayuda?
            </a>
          )}
          <button
            className="theme-toggle-link"
            onClick={toggleTheme}
            aria-label={resolvedTheme === 'dark' ? 'Cambiar a modo claro' : 'Cambiar a modo oscuro'}
          >
            <span className="material-icons-outlined">
              {resolvedTheme === 'dark' ? 'light_mode' : 'dark_mode'}
            </span>
            {resolvedTheme === 'dark' ? 'Modo claro' : 'Modo oscuro'}
          </button>
          <button className="logout-link" onClick={handleLogout}>
            <span className="material-icons-outlined">logout</span>
            Cerrar Sesión
          </button>
        </div>
      </aside>

      {/* ÁREA DE CONTENIDO */}
      <main className="main-content">
        {/* Envolver la ruta en su propio bloque, en vez de dejarla como
            hijo directo del flex column de abajo: un hijo flex con
            "margin: 0 auto" (el patrón que usan casi todas las páginas
            para centrar su contenido con un max-width) deja de estirarse
            y se encoge a su contenido — este wrapper (sin margin auto)
            sí se estira normal, y la página adentro vuelve a centrarse
            como bloque normal, no como hijo flex. */}
        <div className="main-content-inner">
          <PwaInstallGuide />
          {children ?? <Outlet />}
        </div>
        <Footer />
      </main>
    </div>
  );
};