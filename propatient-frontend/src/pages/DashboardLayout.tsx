import React from 'react';
import { Outlet, Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import './DashboardLayout.scss';

export const DashboardLayout = () => {
  const { logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  
  // Obtenemos el nombre dinámico si existe en la sesión
  const doctorName = localStorage.getItem('doctor_name') || 'Dr. Alejandro Bueno';
  const initialLetter = doctorName.replace('Dr. ', '').charAt(0).toUpperCase();
  
  const menuItems = [
    { label: 'Dashboard', icon: 'home', route: '/inicio' },
    { label: 'Pacientes', icon: 'people', route: '/pacientes' },
    { label: 'Citas', icon: 'calendar_month', route: '/calendar' },
    { label: 'Perfil', icon: 'settings', route: '/profile' },
  ];

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <div className="dashboard-container">
      {/* BARRA LATERAL (SIDEBAR) */}
      <aside className="sidebar">
        <div>
          <div className="sidebar-header">
            <div className="logo-container">
              {/* SVG CORREGIDO: Atributos limpios sin barras inversas */}
              <svg className="medical-logo" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path fillRule="evenodd" clipRule="evenodd" d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
              </svg>
              <p className="office-name">PROPatient</p>
            </div>
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
          <button className="logout-link" onClick={handleLogout}>
            <span className="material-icons-outlined">logout</span>
            Cerrar Sesión
          </button>
        </div>
      </aside>

      {/* ÁREA DE CONTENIDO */}
      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
};