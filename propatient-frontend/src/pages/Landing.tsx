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

// Precios y estado de la promo de lanzamiento (ver GetPublicPricing en el
// backend) — sin sesión, alimenta la sección de planes de abajo. Los
// montos son solo informativos (billing.Individual*/Clinic*PriceMXN); el
// cobro real lo definen los Price configurados en Stripe.
interface PublicPricing {
  individual: {
    launchPromoActive: boolean;
    launchPriceMXN: number;
    regularPriceMXN: number;
  };
  clinic: {
    launchPromoActive: boolean;
    launchBasePriceMXN: number;
    regularBasePriceMXN: number;
    launchExtraPriceMXN: number;
    regularExtraPriceMXN: number;
    baseIncludedDoctors: number;
  };
  launchPromoEndsAt: string | null;
}

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
  const [pricing, setPricing] = useState<PublicPricing | null>(null);

  useEffect(() => {
    api.get('/public/pricing')
      .then((res) => setPricing(res.data))
      .catch(() => setPricing(null));
  }, []);

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

      <section className="landing-pricing">
        <h2>¿Eres doctor?</h2>
        <p className="pricing-intro">
          Gestiona tu agenda, tus pacientes y tu expediente clínico digital, y aparece
          en el directorio para que nuevos pacientes te encuentren.
        </p>

        {pricing && (
          <div className="pricing-cards">
            <div className="pricing-card">
              <h3>Plan Individual</h3>
              <div className="pricing-price">
                {pricing.individual.launchPromoActive && (
                  <span className="pricing-price-regular">
                    ${pricing.individual.regularPriceMXN.toLocaleString('es-MX')} MXN/mes
                  </span>
                )}
                <span className="pricing-price-main">
                  ${(pricing.individual.launchPromoActive
                    ? pricing.individual.launchPriceMXN
                    : pricing.individual.regularPriceMXN
                  ).toLocaleString('es-MX')}{' '}
                  MXN/mes
                </span>
              </div>
              {pricing.individual.launchPromoActive && (
                <span className="pricing-badge">Precio de lanzamiento — oferta por tiempo limitado</span>
              )}
              <ul className="pricing-features">
                <li>10 días de prueba gratis</li>
                <li>Agenda de citas online</li>
                <li>Notificaciones automáticas por WhatsApp</li>
                <li>Expediente clínico digital</li>
                <li>Perfil público en el directorio</li>
              </ul>
              {isAuthenticated ? (
                <Link to="/inicio" className="btn-primary-lg">Ir a mi panel</Link>
              ) : (
                <Link to="/login" className="btn-primary-lg">Crear mi cuenta</Link>
              )}
            </div>

            <div className="pricing-card">
              <h3>Plan Clínica</h3>
              <div className="pricing-price">
                {pricing.clinic.launchPromoActive && (
                  <span className="pricing-price-regular">
                    ${pricing.clinic.regularBasePriceMXN.toLocaleString('es-MX')} MXN/mes
                  </span>
                )}
                <span className="pricing-price-main">
                  ${(pricing.clinic.launchPromoActive
                    ? pricing.clinic.launchBasePriceMXN
                    : pricing.clinic.regularBasePriceMXN
                  ).toLocaleString('es-MX')}{' '}
                  MXN/mes
                </span>
              </div>
              <p className="pricing-extra-note">
                Incluye hasta {pricing.clinic.baseIncludedDoctors} personas · $
                {(pricing.clinic.launchPromoActive
                  ? pricing.clinic.launchExtraPriceMXN
                  : pricing.clinic.regularExtraPriceMXN
                ).toLocaleString('es-MX')}{' '}
                MXN/mes por persona adicional
              </p>
              {pricing.clinic.launchPromoActive && (
                <span className="pricing-badge">Precio de lanzamiento — oferta por tiempo limitado</span>
              )}
              <ul className="pricing-features">
                <li>Todo lo del plan individual</li>
                <li>Varios doctores en un solo consultorio</li>
                <li>Personal de recepción con su propio acceso</li>
                <li>Un solo directorio para toda la clínica</li>
              </ul>
              {isAuthenticated ? (
                <Link to="/inicio" className="btn-primary-lg">Ir a mi panel</Link>
              ) : (
                <Link to="/login" className="btn-primary-lg">Crear mi cuenta</Link>
              )}
            </div>
          </div>
        )}
      </section>

      <section className="landing-features">
        <h2>¿Quieres saber más?</h2>
        <div className="features-grid">
          <div className="feature-item">
            <span className="material-icons-outlined">event_available</span>
            <h4>Agendado de citas</h4>
            <p>Tus pacientes agendan solos desde tu perfil público — tú confirmas o rechazas cada solicitud.</p>
          </div>
          <div className="feature-item">
            <span className="material-icons-outlined">chat</span>
            <h4>WhatsApp automático</h4>
            <p>Confirmaciones, recordatorios 24 horas antes y avisos de cancelación, sin que muevas un dedo.</p>
          </div>
          <div className="feature-item">
            <span className="material-icons-outlined">qr_code_2</span>
            <h4>QR para tu consultorio</h4>
            <p>Tu paciente escanea un código y sube sus documentos antes de la consulta, sin que tengas que pedirlos tú.</p>
          </div>
          <div className="feature-item">
            <span className="material-icons-outlined">folder_shared</span>
            <h4>Expediente clínico digital</h4>
            <p>Notas de consulta, historial completo y recetas en PDF, todo en un solo lugar.</p>
          </div>
          <div className="feature-item">
            <span className="material-icons-outlined">travel_explore</span>
            <h4>Apareces en el directorio</h4>
            <p>Pacientes nuevos te encuentran por especialidad o ubicación, sin costo extra.</p>
          </div>
        </div>
      </section>

      <Footer />
    </div>
  );
};
