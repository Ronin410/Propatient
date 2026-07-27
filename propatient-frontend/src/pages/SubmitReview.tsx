import React, { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout';
import { getErrorMessage } from '../utils/errorMessage';
import './Login.scss';

interface ReviewInviteInfo {
  doctorName: string;
  patientName: string;
  alreadyRated: boolean;
}

const cardStyle: React.CSSProperties = {
  padding: '40px 35px',
  backgroundColor: 'var(--bg)',
  borderRadius: '24px',
  boxShadow: '0 20px 40px rgba(0, 0, 0, 0.04), 0 1px 3px rgba(0, 0, 0, 0.02)',
  boxSizing: 'border-box',
  width: '100%'
};

export const SubmitReview: React.FC = () => {
  const { token } = useParams<{ token: string }>();

  const [invite, setInvite] = useState<ReviewInviteInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [rating, setRating] = useState(0);
  const [hoverRating, setHoverRating] = useState(0);
  const [comment, setComment] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    if (!token) return;
    api.get(`/public/reviews/${token}`)
      .then((res) => setInvite(res.data))
      .catch((err) => setLoadError(getErrorMessage(err, 'Este link de reseña no es válido.')))
      .finally(() => setLoading(false));
  }, [token]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);

    if (rating < 1) {
      setFormError('Selecciona una calificación de 1 a 5 estrellas.');
      return;
    }

    setSubmitting(true);
    try {
      await api.post(`/public/reviews/${token}`, { rating, comment });
      setSuccess(true);
    } catch (err: unknown) {
      setFormError(getErrorMessage(err, 'No se pudo enviar tu reseña. Intenta de nuevo.'));
    } finally {
      setSubmitting(false);
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

  if (success || invite.alreadyRated) {
    return (
      <AuthLayout>
        <div className="card" style={cardStyle}>
          <h1 style={{ fontSize: '22px', fontWeight: 700, color: 'var(--color-heading)', textAlign: 'center', marginTop: 0 }}>
            ¡Gracias por tu reseña!
          </h1>
          <p style={{ color: 'var(--color-secondary)', textAlign: 'center' }}>
            {success
              ? 'Tu opinión nos ayuda mucho, y le ayuda a otros pacientes a conocer a su doctor.'
              : 'Ya habías enviado tu calificación de esta consulta.'}
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
            ¿Cómo fue tu consulta con Dr(a). {invite.doctorName}?
          </p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          {formError && (
            <div
              style={{
                padding: '12px 14px', borderRadius: '8px',
                backgroundColor: 'var(--color-danger-bg)', color: 'var(--color-danger)',
                fontSize: '13px', border: '1px solid var(--color-danger-border)'
              }}
            >
              {formError}
            </div>
          )}

          <div style={{ display: 'flex', justifyContent: 'center', gap: '6px' }}>
            {[1, 2, 3, 4, 5].map((n) => (
              <button
                key={n}
                type="button"
                onClick={() => setRating(n)}
                onMouseEnter={() => setHoverRating(n)}
                onMouseLeave={() => setHoverRating(0)}
                aria-label={`${n} estrellas`}
                style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
              >
                <span
                  className="material-icons-outlined"
                  style={{
                    fontSize: '36px',
                    color: n <= (hoverRating || rating) ? 'var(--color-warning)' : 'var(--border)'
                  }}
                >
                  star
                </span>
              </button>
            ))}
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: 'var(--color-heading)', marginBottom: '6px' }}>
              Comentario (opcional)
            </label>
            <textarea
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              rows={4}
              placeholder="Cuéntanos cómo fue tu experiencia..."
              style={{ width: '100%', padding: '10px 14px', borderRadius: '8px', border: '1px solid var(--border)', fontSize: '14px', boxSizing: 'border-box', resize: 'vertical', fontFamily: 'inherit' }}
            />
          </div>

          <button type="submit" className="btn-primary login-button" disabled={submitting}>
            {submitting ? 'Enviando...' : 'Enviar reseña'}
          </button>
        </form>
      </div>
    </AuthLayout>
  );
};
