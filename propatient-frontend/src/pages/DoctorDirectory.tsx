import React, { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import L from 'leaflet';
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';
import 'leaflet/dist/leaflet.css';
import api from '../api/axios';
import { toAbsoluteFileUrl } from '../utils/fileUrl';
import type { PublicDoctor } from '../types';
import { Footer } from '../components/Footer';
import logo from '../assets/logo.png';
import './DoctorDirectory.scss';

// Vite empaqueta los PNG del propio paquete leaflet como assets; sin esto,
// el ícono por defecto de los marcadores sale roto (URLs relativas que
// apuntan al servidor de assets equivocado).
const defaultIcon = L.icon({
  iconUrl: markerIcon,
  iconRetinaUrl: markerIcon2x,
  shadowUrl: markerShadow,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
});

const DEFAULT_CENTER: [number, number] = [23.6345, -102.5528]; // Centro geográfico de México
const DEFAULT_ZOOM = 5;

export const DoctorDirectory: React.FC = () => {
  const [doctors, setDoctors] = useState<PublicDoctor[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  useEffect(() => {
    api.get('/public/doctors')
      .then((res) => setDoctors(res.data || []))
      .catch(() => setDoctors([]))
      .finally(() => setLoading(false));
  }, []);

  const filteredDoctors = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return doctors;
    return doctors.filter((d) =>
      d.fullName.toLowerCase().includes(q) ||
      (d.medicalSpecialty || '').toLowerCase().includes(q) ||
      (d.address || '').toLowerCase().includes(q)
    );
  }, [doctors, search]);

  const doctorsWithLocation = filteredDoctors.filter((d) => d.latitude != null && d.longitude != null);

  const mapCenter: [number, number] = doctorsWithLocation.length > 0
    ? [doctorsWithLocation[0].latitude as number, doctorsWithLocation[0].longitude as number]
    : DEFAULT_CENTER;
  const mapZoom = doctorsWithLocation.length > 0 ? 12 : DEFAULT_ZOOM;

  return (
    <div className="directory-page">
      <header className="directory-nav">
        <Link to="/" className="directory-logo">
          <img src={logo} alt="ProPatient" className="brand-logo-icon" />
          ProPatient
        </Link>
        <Link to="/login" className="nav-link">Soy doctor</Link>
      </header>

      <div className="directory-header">
        <h1>Directorio de doctores</h1>
        <p>Busca por nombre, especialidad o ubicación y agenda tu cita en línea.</p>
        <input
          type="text"
          className="directory-search"
          placeholder="Ej. cardiólogo, Ciudad de México, Dra. Pérez..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>

      <div className="directory-body">
        <div className="directory-map">
          <MapContainer center={mapCenter} zoom={mapZoom} scrollWheelZoom style={{ height: '100%', width: '100%' }}>
            <TileLayer
              attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
              url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
            />
            {doctorsWithLocation.map((doc) => (
              <Marker key={doc.id} position={[doc.latitude as number, doc.longitude as number]} icon={defaultIcon}>
                <Popup>
                  <strong>Dr(a). {doc.fullName}</strong>
                  <br />
                  {doc.medicalSpecialty || 'Médico General'}
                  <br />
                  <Link to={`/dr/${doc.publicSlug}`}>Ver perfil y agendar</Link>
                </Popup>
              </Marker>
            ))}
          </MapContainer>
        </div>

        <div className="directory-list">
          {loading ? (
            <p className="directory-empty">Cargando doctores...</p>
          ) : filteredDoctors.length === 0 ? (
            <p className="directory-empty">No encontramos doctores que coincidan con tu búsqueda.</p>
          ) : (
            filteredDoctors.map((doc) => (
              <Link to={`/dr/${doc.publicSlug}`} key={doc.id} className="directory-card">
                <div className="directory-card-avatar">
                  {doc.avatarUrl ? (
                    <img src={toAbsoluteFileUrl(doc.avatarUrl)} alt={doc.fullName} />
                  ) : (
                    <span className="material-icons-outlined">person</span>
                  )}
                </div>
                <div className="directory-card-info">
                  <h3>Dr(a). {doc.fullName}</h3>
                  <p className="directory-card-specialty">{doc.medicalSpecialty || 'Médico General'}</p>
                  {doc.address && (
                    <p className="directory-card-address">
                      <span className="material-icons-outlined">place</span>
                      {doc.address}
                    </p>
                  )}
                </div>
              </Link>
            ))
          )}
        </div>
      </div>

      <Footer />
    </div>
  );
};
