import React, { useEffect, useState } from 'react';
import api from '../api/axios';
import { getErrorMessage } from '../utils/errorMessage';
import type { Review } from '../types';
import './Reviews.scss';

const Stars: React.FC<{ rating: number }> = ({ rating }) => (
  <span className="stars" aria-label={`${rating} de 5 estrellas`}>
    {[1, 2, 3, 4, 5].map((n) => (
      <span key={n} className={`material-icons-outlined ${n <= rating ? 'filled' : ''}`}>star</span>
    ))}
  </span>
);

export const Reviews: React.FC = () => {
  const [reviews, setReviews] = useState<Review[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchReviews = async () => {
    setLoading(true);
    try {
      const res = await api.get('/reviews');
      setReviews(res.data || []);
    } catch (err) {
      setError(getErrorMessage(err, 'No se pudieron cargar las reseñas.'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchReviews();
  }, []);

  const handleSetApproval = async (review: Review, approved: boolean) => {
    try {
      await api.put(`/reviews/${review.id}/approve`, { approved });
      setReviews((prev) => prev.map((r) => (r.id === review.id ? { ...r, approved } : r)));
    } catch (err) {
      setError(getErrorMessage(err, 'No se pudo actualizar la reseña.'));
    }
  };

  const pending = reviews.filter((r) => !r.approved);
  const approved = reviews.filter((r) => r.approved);

  return (
    <div className="reviews-container">
      <header className="page-header">
        <div>
          <h1>Reseñas de Pacientes</h1>
          <p className="subtitle">
            Al marcar una consulta como completada, se le manda al paciente un WhatsApp para que la califique.
            Revisa y aprueba las reseñas antes de que aparezcan en tu perfil público.
          </p>
        </div>
      </header>

      {error && <div className="reviews-alert">{error}</div>}

      {loading ? (
        <p className="empty-msg">Cargando...</p>
      ) : reviews.length === 0 ? (
        <p className="empty-msg">Todavía no tienes reseñas. Se generan automáticamente al completar una consulta.</p>
      ) : (
        <>
          <section className="card">
            <h3>Pendientes de aprobar ({pending.length})</h3>
            {pending.length === 0 ? (
              <p className="empty-msg">No hay reseñas pendientes.</p>
            ) : (
              <div className="review-list">
                {pending.map((r) => (
                  <div className="review-row" key={r.id}>
                    <div className="review-main">
                      <div className="review-header">
                        <strong>{r.patientFirstName} {r.patientLastName}</strong>
                        <Stars rating={r.rating} />
                      </div>
                      {r.comment && <p className="review-comment">"{r.comment}"</p>}
                    </div>
                    <div className="review-actions">
                      <button className="btn-outline-sm" onClick={() => handleSetApproval(r, true)}>
                        Aprobar y publicar
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="card">
            <h3>Publicadas en tu perfil ({approved.length})</h3>
            {approved.length === 0 ? (
              <p className="empty-msg">Aún no has aprobado ninguna.</p>
            ) : (
              <div className="review-list">
                {approved.map((r) => (
                  <div className="review-row" key={r.id}>
                    <div className="review-main">
                      <div className="review-header">
                        <strong>{r.patientFirstName} {r.patientLastName}</strong>
                        <Stars rating={r.rating} />
                      </div>
                      {r.comment && <p className="review-comment">"{r.comment}"</p>}
                    </div>
                    <div className="review-actions">
                      <button className="btn-outline-sm danger" onClick={() => handleSetApproval(r, false)}>
                        Quitar de mi perfil
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
};
