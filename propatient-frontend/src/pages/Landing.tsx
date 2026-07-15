import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import api from '../api/axios';
import { toAbsoluteFileUrl } from '../utils/fileUrl';
import type { PublicDoctor } from '../types';
import { Footer } from '../components/Footer';
import './Landing.scss';

const ROTATION_MS = 5000;

// Elige un índice al azar distinto del actual (si hay más de un doctor),
// para que la rotación se sienta aleatoria de verdad y no repita seguido
// al mismo doctor.
function pickNextIndex(length: number, current: number): number {
  if (length <= 1) return 0;
  let next = current;
  while (next === current) {
    next = Math.floor(Math.random() * length);
  }
  return next;
}

export const Landing: React.FC = () => {
  const [doctors, setDoctors] = useState<PublicDoctor[]>([]);
  const [activeIndex, setActiveIndex] = useState(0);

  useEffect(() => {
    api.get('/public/doctors')
      .then((res) => setDoctors(res.data || []))
      .catch(() => setDoctors([]));
  }, []);

  useEffect(() => {
    if (doctors.length <= 1) return;
    const timer = setInterval(() => {
      setActiveIndex((prev) => pickNextIndex(doctors.length, prev));
    }, ROTATION_MS);
    return () => clearInterval(timer);
  }, [doctors.length]);

  const activeDoctor = doctors[activeIndex];

  return (
    <div className="landing-page">
      <header className="landing-nav">
        <div className="landing-nav-inner">
          <div className="landing-logo">
            <span className="material-icons-outlined">favorite</span>
            ProPatient
          </div>
          <nav>
            <Link to="/doctores" className="nav-link">Buscar doctor</Link>
            <Link to="/login" className="nav-link">Soy doctor</Link>
            <Link to="/login" className="btn-nav-cta">Iniciar sesión</Link>
          </nav>
        </div>
      </header>

      <section className="landing-hero">
        <div className="hero-copy">
          <h1>Encuentra a tu doctor y agenda tu cita en minutos</h1>
          <p>
            ProPatient conecta pacientes con consultorios médicos: busca por especialidad
            o ubicación, revisa el perfil del doctor y solicita tu cita en línea, sin
            necesidad de crear una cuenta.
          </p>
          <div className="hero-actions">
            <Link to="/doctores" className="btn-primary-lg">Ver directorio de doctores</Link>
            <Link to="/login" className="btn-outline-lg">Soy doctor, quiero unirme</Link>
          </div>
        </div>

        <div className="hero-doctor-card">
          {activeDoctor ? (
            <div key={activeDoctor.id} className="rotating-doctor-card">
              <div className="doctor-avatar">
                {activeDoctor.avatarUrl ? (
                  <img src={toAbsoluteFileUrl(activeDoctor.avatarUrl)} alt={activeDoctor.fullName} />
                ) : (
                  <span className="material-icons-outlined">person</span>
                )}
              </div>
              <h3>Dr(a). {activeDoctor.fullName}</h3>
              <p className="doctor-specialty">{activeDoctor.medicalSpecialty || 'Médico General'}</p>
              {activeDoctor.publicBio && <p className="doctor-bio">{activeDoctor.publicBio}</p>}
              <Link to={`/dr/${activeDoctor.publicSlug}`} className="btn-text">Ver perfil y agendar →</Link>
            </div>
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
        <Link to="/login" className="btn-primary-lg">Crear mi cuenta</Link>
      </section>

      <Footer />
    </div>
  );
};
