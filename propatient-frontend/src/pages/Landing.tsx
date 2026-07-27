import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import api from '../api/axios';
import { toAbsoluteFileUrl } from '../utils/fileUrl';
import type { PublicDoctor } from '../types';
import { Footer } from '../components/Footer';
import { InstallPwaButton } from '../components/InstallPwaButton';
import { useAuth } from '../context/AuthContext';
import logo from '../assets/logo.png';
import './Landing.scss';

const ROTATION_MS = 5000;
// Cuántas tarjetas se muestran a la vez (se acomodan en una sola columna
// en móvil vía CSS, sin cambiar esta lógica) — antes solo se mostraba UN
// doctor a la vez, rotando; ahora se ve un grupo completo y el grupo
// siguiente entra cuando hay más doctores de los que caben en pantalla.
const CARDS_PER_VIEW = 3;

export const Landing: React.FC = () => {
  const { isAuthenticated } = useAuth();
  const [doctors, setDoctors] = useState<PublicDoctor[]>([]);
  const [groupStart, setGroupStart] = useState(0);

  useEffect(() => {
    api.get('/public/doctors')
      .then((res) => setDoctors(res.data || []))
      .catch(() => setDoctors([]));
  }, []);

  useEffect(() => {
    if (doctors.length <= CARDS_PER_VIEW) return;
    const timer = setInterval(() => {
      setGroupStart((prev) => (prev + CARDS_PER_VIEW) % doctors.length);
    }, ROTATION_MS);
    return () => clearInterval(timer);
  }, [doctors.length]);

  // Grupo visible: los siguientes CARDS_PER_VIEW doctores a partir de
  // groupStart, dando la vuelta al principio de la lista si hace falta —
  // así siempre se ven CARDS_PER_VIEW tarjetas completas, nunca un grupo
  // incompleto al llegar al final.
  const visibleDoctors = doctors.length <= CARDS_PER_VIEW
    ? doctors
    : Array.from({ length: CARDS_PER_VIEW }, (_, i) => doctors[(groupStart + i) % doctors.length]);

  return (
    <div className="landing-page">
      <header className="landing-nav">
        <div className="landing-nav-inner">
          <div className="landing-logo">
            <img src={logo} alt="ProPatient Clinic" className="brand-logo-icon" />
            ProPatient Clinic
          </div>
          <nav>
            <Link to="/doctores" className="nav-link">Buscar doctor</Link>
            <InstallPwaButton />
            {isAuthenticated ? (
              <Link to="/inicio" className="btn-nav-cta">Ir a mi panel</Link>
            ) : (
              <>
                <Link to="/login" className="nav-link">Soy doctor</Link>
                <Link to="/login" className="btn-nav-cta">Iniciar sesión</Link>
              </>
            )}
          </nav>
        </div>
      </header>

      <section className="landing-hero">
        <div className="hero-copy">
          <h1>Encuentra a tu doctor y agenda tu cita en minutos</h1>
          <p>
            ProPatient Clinic conecta pacientes con consultorios médicos: busca por especialidad
            o ubicación, revisa el perfil del doctor y solicita tu cita en línea, sin
            necesidad de crear una cuenta.
          </p>
          <div className="hero-actions">
            <Link to="/doctores" className="btn-primary-lg">Ver directorio de doctores</Link>
            {isAuthenticated ? (
              <Link to="/inicio" className="btn-outline-lg">Ir a mi panel</Link>
            ) : (
              <Link to="/login" className="btn-outline-lg">Soy doctor, quiero unirme</Link>
            )}
          </div>
        </div>
      </section>

      <section className="landing-featured-doctors">
        <h2>Doctores en ProPatient Clinic</h2>
        <div className="hero-doctor-cards">
          {visibleDoctors.length > 0 ? (
            visibleDoctors.map((doc) => (
              <div key={doc.id} className="rotating-doctor-card">
                <div className="doctor-avatar">
                  {doc.avatarUrl ? (
                    <img src={toAbsoluteFileUrl(doc.avatarUrl)} alt={doc.fullName} />
                  ) : (
                    <span className="material-icons-outlined">person</span>
                  )}
                </div>
                <h3>Dr(a). {doc.fullName}</h3>
                <p className="doctor-specialty">{doc.medicalSpecialty || 'Médico General'}</p>
                {doc.publicBio && <p className="doctor-bio">{doc.publicBio}</p>}
                <Link to={`/dr/${doc.publicSlug}`} className="btn-text">Ver perfil y agendar →</Link>
              </div>
            ))
          ) : (
            <div className="rotating-doctor-card empty">
              <span className="material-icons-outlined">groups</span>
              <p>Pronto verás aquí a los doctores de tu zona.</p>
            </div>
          )}
        </div>
      </section>

      <section className="landing-how">
        <h2>¿Cómo funciona?</h2>
        <div className="how-steps">
          <div className="how-step">
            <span className="step-number">1</span>
            <h4>Busca un doctor</h4>
            <p>Explora el directorio por especialidad o ubicación en el mapa.</p>
          </div>
          <div className="how-step">
            <span className="step-number">2</span>
            <h4>Solicita tu cita</h4>
            <p>Llena tus datos y el horario que prefieras, sin crear una cuenta.</p>
          </div>
          <div className="how-step">
            <span className="step-number">3</span>
            <h4>El consultorio confirma</h4>
            <p>El doctor revisa tu solicitud y te confirma por correo o teléfono.</p>
          </div>
        </div>
      </section>

      <section className="landing-doctors-cta">
        <h2>¿Eres doctor?</h2>
        <p>
          Gestiona tu agenda, tus pacientes y tu expediente clínico digital, y aparece
          en el directorio para que nuevos pacientes te encuentren.
        </p>
        {isAuthenticated ? (
          <Link to="/inicio" className="btn-primary-lg">Ir a mi panel</Link>
        ) : (
          <Link to="/login" className="btn-primary-lg">Crear mi cuenta</Link>
        )}
      </section>

      <Footer />
    </div>
  );
};
