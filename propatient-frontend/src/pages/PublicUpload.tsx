import React, { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout';
import { getErrorMessage } from '../utils/errorMessage';
import { formatToLocalDate, formatToLocalTime } from '../utils/dateFormatter';
import './Login.scss';

interface UploadInviteInfo {
  patientName: string;
  doctorName: string;
  appointmentDateTime: string;
  documentCount: number;
}

const cardStyle: React.CSSProperties = {
  padding: '40px 35px',
  backgroundColor: 'var(--bg)',
  borderRadius: '24px',
  boxShadow: '0 20px 40px rgba(0, 0, 0, 0.04), 0 1px 3px rgba(0, 0, 0, 0.02)',
  boxSizing: 'border-box',
  width: '100%'
};

// Pantalla a la que llega el paciente al escanear el QR de "sube tus
// documentos antes de la cita" (ver ConsultationManager → toggleQR): sin
// sesión, solo el token en la URL. Deja los archivos ya cargados para
// cuando el doctor abra la consulta, sin que el paciente tenga que crear
// una cuenta ni recordar llevar copias físicas.
export const PublicUpload: React.FC = () => {
  const { token } = useParams<{ token: string }>();

  const [invite, setInvite] = useState<UploadInviteInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [files, setFiles] = useState<File[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [savedCount, setSavedCount] = useState(0);

  useEffect(() => {
    if (!token) return;
    api.get(`/public/upload/${token}`)
      .then((res) => setInvite(res.data))
      .catch((err) => setLoadError(getErrorMessage(err, 'Este link no es válido o ya venció.')))
      .finally(() => setLoading(false));
  }, [token]);

  const handleFilesChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFiles(Array.from(e.target.files || []));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (files.length === 0) {
      setUploadError('Selecciona al menos un archivo o foto.');
      return;
    }
    setUploading(true);
    setUploadError(null);
    try {
      const formData = new FormData();
      files.forEach((f) => formData.append('files', f));
      const res = await api.post(`/public/upload/${token}`, formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      setSavedCount(res.data?.saved || 0);
      setFiles([]);
    } catch (err: unknown) {
      setUploadError(getErrorMessage(err, 'No se pudieron subir tus archivos. Intenta de nuevo.'));
    } finally {
      setUploading(false);
    }
  };

  if (loading) {
    return (
      <AuthLayout>
        <div className="card" style={cardStyle}>
          <p style={{ textAlign: 'center', color: 'var(--color-secondary)' }}>Cargando...</p>
        </div>
      </AuthLayout>
    );
  }

  if (loadError || !invite) {
    return (
      <AuthLayout>
        <div className="card" style={cardStyle}>
          <h1 style={{ fontSize: '22px', fontWeight: 700, color: 'var(--color-heading)', textAlign: 'center', marginTop: 0 }}>
            Link no disponible
          </h1>
          <p style={{ color: 'var(--color-secondary)', textAlign: 'center' }}>{loadError}</p>
          <p style={{ textAlign: 'center', marginTop: '20px' }}>
            <Link to="/" style={{ color: 'var(--color-primary)', fontWeight: 600 }}>Ir a ProPatient Clinic</Link>
          </p>
        </div>
      </AuthLayout>
    );
  }

  if (savedCount > 0) {
    return (
      <AuthLayout>
        <div className="card" style={cardStyle}>
          <h1 style={{ fontSize: '22px', fontWeight: 700, color: 'var(--color-heading)', textAlign: 'center', marginTop: 0 }}>
            ¡Listo!
          </h1>
          <p style={{ color: 'var(--color-secondary)', textAlign: 'center' }}>
            Subiste {savedCount} {savedCount === 1 ? 'archivo' : 'archivos'}. El consultorio ya los tiene disponibles para tu cita.
          </p>
          <p style={{ textAlign: 'center', marginTop: '20px' }}>
            <button
              type="button"
              className="btn-primary login-button"
              onClick={() => setSavedCount(0)}
              style={{ maxWidth: '260px', margin: '0 auto' }}
            >
              Subir más archivos
            </button>
          </p>
        </div>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <div className="card" style={cardStyle}>
        <div style={{ textAlign: 'center', marginBottom: '28px' }}>
          <h1 style={{ fontSize: '24px', fontWeight: 700, color: 'var(--color-heading)', margin: 0 }}>
            ¡Hola, {invite.patientName}!
          </h1>
          <p style={{ fontSize: '14px', color: 'var(--color-secondary)', marginTop: '6px' }}>
            Sube tus estudios o documentos para tu cita con Dr(a). {invite.doctorName}
            {invite.appointmentDateTime && (
              <> el {formatToLocalDate(invite.appointmentDateTime)} a las {formatToLocalTime(invite.appointmentDateTime)}</>
            )}
            . Así el consultorio ya los tiene listos cuando llegues.
          </p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          {uploadError && (
            <div
              style={{
                padding: '12px 14px', borderRadius: '8px',
                backgroundColor: 'var(--color-danger-bg)', color: 'var(--color-danger)',
                fontSize: '13px', border: '1px solid var(--color-danger-border)'
              }}
            >
              {uploadError}
            </div>
          )}

          <div>
            <label style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: 'var(--color-heading)', marginBottom: '6px' }}>
              Fotos o documentos (PDF o imagen, hasta 15 MB cada uno)
            </label>
            <input
              type="file"
              accept="application/pdf,image/*"
              multiple
              onChange={handleFilesChange}
              style={{ width: '100%', padding: '10px 0', fontSize: '14px' }}
            />
            {files.length > 0 && (
              <p style={{ fontSize: '13px', color: 'var(--color-secondary)', marginTop: '6px' }}>
                {files.length} {files.length === 1 ? 'archivo seleccionado' : 'archivos seleccionados'}
              </p>
            )}
          </div>

          <button type="submit" className="btn-primary login-button" disabled={uploading || files.length === 0}>
            {uploading ? 'Subiendo...' : 'Subir archivos'}
          </button>

          {invite.documentCount > 0 && (
            <p style={{ fontSize: '12px', color: 'var(--color-secondary)', textAlign: 'center', margin: 0 }}>
              Ya hay {invite.documentCount} {invite.documentCount === 1 ? 'archivo cargado' : 'archivos cargados'} para esta cita.
            </p>
          )}
        </form>
      </div>
    </AuthLayout>
  );
};
