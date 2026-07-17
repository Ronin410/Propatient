import React, { useEffect, useState } from 'react';
import { Outlet, Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useTheme } from '../context/ThemeContext';
import { Footer } from '../components/Footer';
import api from '../api/axios';
import './DashboardLayout.scss';

interface DashboardLayoutProps {
  // Si se pasan children, se muestran en vez del <Outlet /> de la ruta
  // anidada — usado por App.tsx para reutilizar el layout (con menú
  // lateral) en páginas que no son rutas hijas del dashboard, como
  // /privacidad cuando hay sesión iniciada.
  children?: React.ReactNode;
}

export const DashboardLayout: React.FC<DashboardLayoutProps> = ({ children }) => {
  const { logout, isStaff, doctorName: sessionDoctorName, setDoctorName } = useAuth();
  const { resolvedTheme, toggleTheme } = useTheme();
  const location = useLocation();
  const navigate = useNavigate();
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

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
      { label: 'Facturación', icon: 'credit_card', route: '/billing' },
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
              {/* SVG CORREGIDO: Atributos limpios sin barras inversas */}
              <svg className="medical-logo" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path fillRule="evenodd" clipRule="evenodd" d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
              </svg>
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
        {children ?? <Outlet />}
        <Footer />
      </main>
    </div>
  );
};