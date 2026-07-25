import React, { useEffect, useState } from 'react';
import { MapContainer, TileLayer, Marker, useMap, useMapEvents } from 'react-leaflet';
import L from 'leaflet';
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';
import 'leaflet/dist/leaflet.css';
import api from '../api/axios';
import { getErrorMessage } from '../utils/errorMessage';
import './LocationPicker.scss';

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

interface LocationPickerProps {
  address: string;
  latitude: number | null;
  longitude: number | null;
  onChange: (lat: number, lng: number) => void;
}

// Recentra el mapa cuando la posición cambia desde fuera del mapa (ej. tras
// "Ubicar mi dirección"), sin esto Leaflet no mueve la vista solo porque el
// prop del Marker cambió.
const RecenterOnChange: React.FC<{ position: [number, number] | null }> = ({ position }) => {
  const map = useMap();
  useEffect(() => {
    if (position) map.setView(position, Math.max(map.getZoom(), 15));
  }, [position, map]);
  return null;
};

// Permite también fijar la posición haciendo click/tap directo en el mapa,
// además de arrastrar el marcador.
const ClickToPlace: React.FC<{ onPlace: (lat: number, lng: number) => void }> = ({ onPlace }) => {
  useMapEvents({
    click(e) {
      onPlace(e.latlng.lat, e.latlng.lng);
    },
  });
  return null;
};

export const LocationPicker: React.FC<LocationPickerProps> = ({ address, latitude, longitude, onChange }) => {
  const [locating, setLocating] = useState(false);
  const [locateError, setLocateError] = useState<string | null>(null);

  const hasPosition = latitude != null && longitude != null;
  const position: [number, number] | null = hasPosition ? [latitude as number, longitude as number] : null;

  const handleLocateAddress = async () => {
    if (!address.trim()) {
      setLocateError('Escribe primero una dirección para poder ubicarla.');
      return;
    }
    setLocating(true);
    setLocateError(null);
    try {
      const res = await api.get('/doctor/geocode', { params: { address } });
      onChange(res.data.latitude, res.data.longitude);
    } catch (err) {
      setLocateError(getErrorMessage(err, 'No pudimos ubicar esa dirección. Intenta escribirla con más detalle o coloca el pin manualmente.'));
    } finally {
      setLocating(false);
    }
  };

  return (
    <div className="location-picker">
      <div className="location-picker-actions">
        <button type="button" className="btn-outline-sm" onClick={handleLocateAddress} disabled={locating}>
          <span className="material-icons-outlined">my_location</span>
          {locating ? 'Ubicando...' : 'Ubicar mi dirección en el mapa'}
        </button>
        <p className="location-picker-hint">
          Luego arrastra el marcador (o haz clic en el mapa) para ajustar la ubicación exacta de tu consultorio.
        </p>
      </div>

      {locateError && <div className="location-picker-error">{locateError}</div>}

      <div className="location-picker-map">
        <MapContainer
          center={position || DEFAULT_CENTER}
          zoom={position ? 15 : 5}
          // Desactivado a propósito: con scroll-to-zoom activo, pasar el
          // mouse sobre el mapa "atrapa" la rueda del mouse/trackpad y
          // hace zoom en vez de dejar que la página sega bajando — dentro
          // de un formulario largo (ver CompleteProfile.tsx) esto se
          // sentía como si el formulario estuviera cortado. Se puede
          // seguir haciendo zoom con los botones +/- o doble clic.
          scrollWheelZoom={false}
          style={{ height: '100%', width: '100%' }}
        >
          <TileLayer
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          />
          <RecenterOnChange position={position} />
          <ClickToPlace onPlace={onChange} />
          {position && (
            <Marker
              position={position}
              icon={defaultIcon}
              draggable
              eventHandlers={{
                dragend: (e) => {
                  const marker = e.target as L.Marker;
                  const { lat, lng } = marker.getLatLng();
                  onChange(lat, lng);
                },
              }}
            />
          )}
        </MapContainer>
        {!position && <div className="location-picker-placeholder">Aún no has ubicado tu consultorio en el mapa.</div>}
      </div>
    </div>
  );
};
