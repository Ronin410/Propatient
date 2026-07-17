import React, { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { MapContainer, TileLayer, Marker } from 'react-leaflet';
import L from 'leaflet';
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';
import 'leaflet/dist/leaflet.css';
import api from '../api/axios';
import { toAbsoluteFileUrl } from '../utils/fileUrl';
import { getErrorMessage } from '../utils/errorMessage';
import { sanitizePhoneInput } from '../utils/phoneInput';
import { preloadRecaptcha, getRecaptchaToken } from '../utils/recaptcha';
import type { PublicDoctor, WeekSchedule } from '../types';
import { Footer } from '../components/Footer';
import './PublicDoctorProfile.scss';

const DAY_ORDER: (keyof WeekSchedule)[] = [
  'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday',
];
const DAY_LABELS: Record<keyof WeekSchedule, string> = {
  monday: 'Lunes', tuesday: 'Martes', wednesday: 'Miércoles', thursday: 'Jueves',
  friday: 'Viernes', saturday: 'Sábado', sunday: 'Domingo',
};

// Líneas legibles del horario configurado, ej. "Lunes: 06:00 - 19:00
// (descanso 14:00-15:00)" — solo los días habilitados, en el orden usual
// de la semana en México (lunes primero).
function formatScheduleLines(schedule: WeekSchedule | null | undefined): string[] {
  if (!schedule) return [];
  return DAY_ORDER
    .filter((day) => schedule[day]?.enabled)
    .map((day) => {
      const d = schedule[day];
      const breaks = d.breaks || [];
      const breaksText = breaks.length
        ? ` (descanso ${breaks.map((b) => `${b.start}-${b.end}`).join(', ')})`
        : '';
      return `${DAY_LABELS[day]}: ${d.start} - ${d.end}${breaksText}`;
    });
}

// Redes sociales / landing page configuradas por el doctor, en el orden
// en que se muestran — solo se listan las que de verdad llenó.
const SOCIAL_LINKS: { key: keyof PublicDoctor; label: string }[] = [
  { key: 'facebookUrl', label: 'Facebook' },
  { key: 'instagramUrl', label: 'Instagram' },
  { key: 'linkedinUrl', label: 'LinkedIn' },
  { key: 'twitterUrl', label: 'X / Twitter' },
  { key: 'tiktokUrl', label: 'TikTok' },
  { key: 'youtubeUrl', label: 'YouTube' },
  { key: 'websiteUrl', label: 'Sitio web' },
];

const defaultIcon = L.icon({
  iconUrl: markerIcon,
  iconRetinaUrl: markerIcon2x,
  shadowUrl: markerShadow,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
});

interface BookingForm {
  appointmentDateTime: string;
  reason: string;
  patientFirstName: string;
  patientLastName: string;
  patientPhone: string;
  patientEmail: string;
  dataConsent: boolean;
}

const EMPTY_FORM: BookingForm = {
  appointmentDateTime: '',
  reason: '',
  patientFirstName: '',
  patientLastName: '',
  patientPhone: '',
  patientEmail: '',
  dataConsent: false,
};

// Formato mínimo aceptado por <input type="datetime-local">: no se puede
// solicitar una cita en el pasado.
function minDateTimeLocal(): string {
  const d = new Date();
  const pad = (n: number) => n.toString().padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export const PublicDoctorProfile: React.FC = () => {
  const { slug } = useParams<{ slug: string }>();
  const [doctor, setDoctor] = useState<PublicDoctor | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);

  const [form, setForm] = useState<BookingForm>(EMPTY_FORM);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (!slug) return;
    setLoading(true);
    api.get(`/public/doctors/${slug}`)
      .then((res) => setDoctor(res.data))
      .catch(() => setNotFound(true))
      .finally(() => setLoading(false));
  }, [slug]);

  useEffect(() => {
    preloadRecaptcha();
  }, []);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { name, value } = e.target;
    setForm((prev) => ({ ...prev, [name]: name === 'patientPhone' ? sanitizePhoneInput(value) : value }));
  };

  const handleConsentChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setForm((prev) => ({ ...prev, dataConsent: e.target.checked }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!doctor) return;
    if (!form.dataConsent) {
      setSubmitError('Debes aceptar el tratamiento de tus datos de salud para poder agendar la cita.');
      return;
    }
    setSubmitting(true);
    setSubmitError(null);
    try {
      const recaptchaToken = await getRecaptchaToken('public_appointment');
      await api.post('/public/appointments', {
        doctorId: doctor.id,
        appointmentDateTime: new Date(form.appointmentDateTime).toISOString(),
        reason: form.reason,
        patientFirstName: form.patientFirstName,
        patientLastName: form.patientLastName,
        patientPhone: form.patientPhone,
        patientEmail: form.patientEmail,
        dataConsent: form.dataConsent,
        recaptchaToken,
      });
      setSubmitted(true);
    } catch (err) {
      setSubmitError(getErrorMessage(err, 'No se pudo enviar tu solicitud de cita. Intenta de nuevo.'));
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="public-profile-page">
        <p className="public-profile-loading">Cargando perfil del doctor...</p>
      </div>
    );
  }

  if (notFound || !doctor) {
    return (
      <div className="public-profile-page">
        <div className="public-profile-notfound">
          <span className="material-icons-outlined">search_off</span>
          <h2>No encontramos este perfil</h2>
          <p>El doctor que buscas no está disponible o el enlace ya no es válido.</p>
          <Link to="/doctores" className="btn-primary-lg">Ver directorio de doctores</Link>
        </div>
      </div>
    );
  }

  const hasLocation = doctor.latitude != null && doctor.longitude != null;

  return (
    <div className="public-profile-page">
      <header className="public-profile-nav">
        <Link to="/" className="public-profile-logo">
          <span className="material-icons-outlined">favorite</span>
          ProPatient
        </Link>
        <Link to="/doctores" className="nav-link">Ver directorio</Link>
      </header>

      <div className="public-profile-body">
        <section className="public-profile-card">
          <div className="public-profile-avatar">
            {doctor.avatarUrl ? (
              <img src={toAbsoluteFileUrl(doctor.avatarUrl)} alt={doctor.fullName} />
            ) : (
              <span className="material-icons-outlined">person</span>
            )}
          </div>
          <h1>Dr(a). {doctor.fullName}</h1>
          <p className="public-profile-specialty">{doctor.medicalSpecialty || 'Médico General'}</p>

          {doctor.publicBio && <p className="public-profile-bio">{doctor.publicBio}</p>}

          <div className="public-profile-contact">
            {doctor.address && (
              <div className="contact-item">
                <span className="material-icons-outlined">place</span>
                {doctor.address}
              </div>
            )}
            {doctor.phone && (
              <div className="contact-item">
                <span className="material-icons-outlined">call</span>
                {doctor.phone}
              </div>
            )}
          </div>

          {SOCIAL_LINKS.some(({ key }) => doctor[key]) && (
            <div className="public-profile-social">
              {SOCIAL_LINKS.filter(({ key }) => doctor[key]).map(({ key, label }) => (
                <a
                  key={key}
                  href={doctor[key] as string}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="social-link"
                >
                  <span className="material-icons-outlined">link</span>
                  {label}
                </a>
              ))}
            </div>
          )}

          {formatScheduleLines(doctor.schedule).length > 0 && (
            <div className="public-profile-schedule">
              <h3>
                <span className="material-icons-outlined">schedule</span> Horario de atención
              </h3>
              <ul>
                {formatScheduleLines(doctor.schedule).map((line) => (
                  <li key={line}>{line}</li>
                ))}
              </ul>
            </div>
          )}

          {hasLocation && (
            <div className="public-profile-map">
              <MapContainer
                center={[doctor.latitude as number, doctor.longitude as number]}
                zoom={15}
                scrollWheelZoom={false}
                style={{ height: '100%', width: '100%' }}
              >
                <TileLayer
                  attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
                  url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
                />
                <Marker position={[doctor.latitude as number, doctor.longitude as number]} icon={defaultIcon} />
              </MapContainer>
            </div>
          )}
        </section>

        <section className="public-profile-booking">
          {submitted ? (
            <div className="booking-confirmation">
              <span className="material-icons-outlined">mark_email_read</span>
              <h2>¡Solicitud enviada!</h2>
              <p>
                Tu solicitud de cita con Dr(a). {doctor.fullName} fue enviada. El consultorio la
                confirmará pronto por teléfono o correo electrónico.
              </p>
              <Link to="/doctores" className="btn-outline-lg">Buscar otro doctor</Link>
            </div>
          ) : (
            <>
              <h2>Solicita tu cita</h2>
              <p className="booking-subtitle">
                Llena tus datos y el horario que prefieras. El consultorio confirmará tu cita antes de que quede agendada.
              </p>
              <form onSubmit={handleSubmit} className="booking-form">
                {submitError && <div className="booking-error">{submitError}</div>}

                <div className="form-row">
                  <div className="form-group">
                    <label>Nombre(s)</label>
                    <input type="text" name="patientFirstName" value={form.patientFirstName} onChange={handleChange} required />
                  </div>
                  <div className="form-group">
                    <label>Apellido(s)</label>
                    <input type="text" name="patientLastName" value={form.patientLastName} onChange={handleChange} required />
                  </div>
                </div>

                <div className="form-row">
                  <div className="form-group">
                    <label>Teléfono</label>
                    <input type="tel" name="patientPhone" value={form.patientPhone} onChange={handleChange} required />
                  </div>
                  <div className="form-group">
                    <label>Correo electrónico</label>
                    <input type="email" name="patientEmail" value={form.patientEmail} onChange={handleChange} required />
                  </div>
                </div>

                <div className="form-group">
                  <label>Fecha y hora deseada</label>
                  <input
                    type="datetime-local"
                    name="appointmentDateTime"
                    value={form.appointmentDateTime}
                    onChange={handleChange}
                    min={minDateTimeLocal()}
                    required
                  />
                </div>

                <div className="form-group">
                  <label>Motivo de la consulta (opcional)</label>
                  <textarea name="reason" value={form.reason} onChange={handleChange} rows={3} />
                </div>

                <label className="consent-checkbox">
                  <input type="checkbox" checked={form.dataConsent} onChange={handleConsentChange} required />
                  <span>
                    Acepto que mis datos personales y de salud sean tratados conforme al{' '}
                    <Link to="/privacidad" target="_blank" rel="noopener noreferrer">Aviso de Privacidad</Link>{' '}
                    para agendar y dar seguimiento a esta cita.
                  </span>
                </label>

                <button type="submit" className="btn-primary-lg" disabled={submitting}>
                  {submitting ? 'Enviando solicitud...' : 'Solicitar cita'}
                </button>
              </form>
            </>
          )}
        </section>
      </div>

      {doctor.galleryImages && doctor.galleryImages.length > 0 && (
        <section className="public-profile-gallery">
          <h2>Galería</h2>
          <div className="gallery-grid">
            {doctor.galleryImages.map((img) => (
              <div className="gallery-thumb" key={img.id}>
                <img src={toAbsoluteFileUrl(img.imagePath)} alt={`Foto de Dr(a). ${doctor.fullName}`} />
              </div>
            ))}
          </div>
        </section>
      )}

      {doctor.reviews && doctor.reviews.length > 0 && (
        <section className="public-profile-reviews">
          <div className="reviews-heading">
            <h2>Opiniones de pacientes</h2>
            {doctor.reviewsAverage != null && (
              <div className="reviews-average">
                <span className="material-icons-outlined">star</span>
                {doctor.reviewsAverage.toFixed(1)} de 5 ({doctor.reviews.length} {doctor.reviews.length === 1 ? 'reseña' : 'reseñas'})
              </div>
            )}
          </div>
          <div className="reviews-list">
            {doctor.reviews.map((r, idx) => (
              <div className="review-card" key={idx}>
                <div className="review-card-header">
                  <strong>{r.patientFirstName}</strong>
                  <span className="review-stars">
                    {[1, 2, 3, 4, 5].map((n) => (
                      <span key={n} className={`material-icons-outlined ${n <= r.rating ? 'filled' : ''}`}>star</span>
                    ))}
                  </span>
                </div>
                {r.comment && <p className="review-comment">"{r.comment}"</p>}
              </div>
            ))}
          </div>
        </section>
      )}

      <Footer />
    </div>
  );
};
