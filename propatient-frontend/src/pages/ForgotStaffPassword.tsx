import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout';
import { getErrorMessage } from '../utils/errorMessage';
import './Login.scss';

// Pide el correo y siempre muestra el mismo mensaje de éxito, exista o no
// la cuenta — el backend (RequestStaffPasswordReset) ya responde así a
// propósito para no revelar qué correos están registrados como personal.
export const ForgotStaffPassword: React.FC = () => {
  const [email, setEmail] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const cardStyle: React.CSSProperties = {
    padding: '40px 35px',
    backgroundColor: 'var(--bg)',
    borderRadius: '24px',
    boxShadow: '0 20px 40px rgba(0, 0, 0, 0.04), 0 1px 3px rgba(0, 0, 0, 0.02)',
    boxSizing: 'border-box',
    width: '100%'
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api.post('/auth/staff-password-reset/request', { email });
      setSent(true);
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo procesar la solicitud. Intenta de nuevo.'));
    } finally {
      setSubmitting(false);
    }
  };

  if (sent) {
    return (
      <AuthLayout>
        <div className="card" style={cardStyle}>
          <h1 style={{ fontSize: '22px', fontWeight: 700, color: 'var(--color-heading)', textAlign: 'center', marginTop: 0 }}>
            Revisa tu correo
          </h1>
          <p style={{ color: 'var(--color-secondary)', textAlign: 'center' }}>
            Si <strong>{email}</strong> está registrado como personal, te enviamos instrucciones para restablecer tu contraseña. El link vence en 1 hora.
          </p>
          <p style={{ textAlign: 'center', marginTop: '20px' }}>
            <Link to="/staff-login" style={{ color: 'var(--color-primary)', fontWeight: 600 }}>Volver a iniciar sesión</Link>
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
            Recuperar contraseña
          </h1>
          <p style={{ fontSize: '14px', color: 'var(--color-secondary)', marginTop: '6px' }}>
            Escribe el correo con el que accedes como personal del consultorio.
          </p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          {error && (
            <div style={{
              padding: '12px 14px', borderRadius: '8px',
              backgroundColor: 'var(--color-danger-bg)', color: 'var(--color-danger)',
              fontSize: '13px', border: '1px solid var(--color-danger-border)'
            }}>
              {error}
            </div>
          )}

          <div>
            <label style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: 'var(--color-heading)', marginBottom: '6px' }}>
              Correo
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
              style={{ width: '100%', padding: '10px 14px', borderRadius: '8px', border: '1px solid var(--border)', fontSize: '14px', boxSizing: 'border-box' }}
            />
          </div>

          <button type="submit" className="btn-primary login-button" disabled={submitting}>
            {submitting ? 'Enviando...' : 'Enviar instrucciones'}
          </button>

          <p style={{ fontSize: '13px', color: 'var(--color-secondary)', textAlign: 'center', marginTop: '8px' }}>
            <Link to="/staff-login" style={{ color: 'var(--color-primary)', fontWeight: 600 }}>Volver a iniciar sesión</Link>
          </p>
        </form>
      </div>
    </AuthLayout>
  );
};
