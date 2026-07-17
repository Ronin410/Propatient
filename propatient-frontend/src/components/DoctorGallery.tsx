import React, { useEffect, useRef, useState } from 'react';
import api from '../api/axios';
import { toAbsoluteFileUrl } from '../utils/fileUrl';
import { getErrorMessage } from '../utils/errorMessage';
import type { GalleryImage } from '../types';
import './DoctorGallery.scss';

const MAX_IMAGES = 8;

// Galería de fotos para el perfil público del doctor (consultorio, equipo,
// el propio doctor) — a diferencia del avatar/logo, sube y borra cada foto
// de inmediato (sin esperar a "Guardar Cambios" del formulario principal),
// porque cada una es su propio registro en el backend, no un campo del
// perfil (ver internal/handlers/gallery_handler.go).
export const DoctorGallery: React.FC = () => {
  const [images, setImages] = useState<GalleryImage[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const fetchImages = async () => {
    try {
      const res = await api.get('/doctor/gallery');
      setImages(res.data || []);
    } catch (err) {
      setError(getErrorMessage(err, 'No se pudieron cargar tus fotos.'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchImages();
  }, []);

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    setError(null);
    try {
      const formData = new FormData();
      formData.append('image', file);
      await api.post('/doctor/gallery', formData, { headers: { 'Content-Type': 'multipart/form-data' } });
      await fetchImages();
    } catch (err) {
      setError(getErrorMessage(err, 'No se pudo subir la foto.'));
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await api.delete(`/doctor/gallery/${id}`);
      setImages((prev) => prev.filter((img) => img.id !== id));
    } catch (err) {
      setError(getErrorMessage(err, 'No se pudo eliminar la foto.'));
    }
  };

  return (
    <div className="doctor-gallery">
      <div className="section-title">
        <h3>Galería de fotos</h3>
        <p>Sube fotos de tu consultorio, tu equipo, o de ti mismo — se muestran en tu perfil público para dar más confianza a pacientes nuevos. Hasta {MAX_IMAGES} fotos.</p>
      </div>

      {error && <div className="doctor-gallery-error">{error}</div>}

      {loading ? (
        <p className="doctor-gallery-loading">Cargando...</p>
      ) : (
        <div className="doctor-gallery-grid">
          {images.map((img) => (
            <div className="doctor-gallery-thumb" key={img.id}>
              <img src={toAbsoluteFileUrl(img.imagePath)} alt="Foto del consultorio" />
              <button type="button" className="btn-remove-photo" onClick={() => handleDelete(img.id)} aria-label="Eliminar foto">
                <span className="material-icons-outlined">delete</span>
              </button>
            </div>
          ))}

          {images.length < MAX_IMAGES && (
            <label className="doctor-gallery-upload">
              <input ref={fileInputRef} type="file" accept="image/*" onChange={handleFileSelect} disabled={uploading} />
              <span className="material-icons-outlined">{uploading ? 'hourglass_top' : 'add_a_photo'}</span>
              <span>{uploading ? 'Subiendo...' : 'Agregar foto'}</span>
            </label>
          )}
        </div>
      )}
    </div>
  );
};
