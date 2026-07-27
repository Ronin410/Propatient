import React, { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout';
import { getErrorMessage } from '../utils/errorMessage';
import './Login.scss';

interface InviteInfo {
  fullName: string;
  email: string;
  doctorName: string;
}

export const AcceptStaffInvite: React.FC = () => {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();

  const [invite, setInvite] = useState<InviteInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    if (!token) return;
    api.get(`/auth/staff-invite/${token}`)
      .then((res) => setInvite(res.data))
      .catch((err) => setLoadError(getErrorMessage(err, 'Esta invitación no es válida.')))
      .finally(() => setLoading(false));
  }, [token]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);

    if (password.length < 6) {
      setFormError('La contraseña debe tener al menos 6 caracteres.');
      return;
    }
    if (password !== confirmPassword) {
      setFormError('Las contraseñas no coinciden.');
      return;
    }

    setSubmitting(true);
    try {
      await api.post(`/auth/staff-invite/${token}`, { password });
      setSuccess(true);
      setTimeout(() => navigate('/staff-login'), 2000);
    } catch (err: unknown) {
      setFormError(getErrorMessage(err, 'No se pudo crear la contraseña. Intenta de nuevo.'));
    } finally {
      setSubmitting(false);
    }
  };

  const cardStyle: React.CSSProperties = {
    padding: '40px 35px',
    backgroundColor: 'var(--bg)',
    borderRadius: '24px',
    boxShadow: '0 20px 40px rgba(0, 0, 0, 0.04), 0 1px 3px rgba(0, 0, 0, 0.02)',
    boxSizing: 'border-box',
    width: '100%'
  };

  if (loading) {
    return (
      <AuthLayout>
        <div className="card" style={cardStyle}>
          <p style={{ textAlign: 'center', color: 'var(--color-secondary)' }}>Verificando invitación...</p>
        </div>
      </AuthLayout>
    );
  }

  if (loadError || !invite) {
    return (
      <AuthLayout>
        <div className="card" style={cardStyle}>
          <h1 style={{ fontSize: '22px', fontWeight: 700, color: 'var(--color-heading)', textAlign: 'center', marginTop: 0 }}>
            Invitación no disponible
          </h1>
          <p style={{ color: 'var(--color-secondary)', textAlign: 'center' }}>{loadError}</p>
          <p style={{ textAlign: 'center', marginTop: '20px' }}>
            <Link to="/staff-login" style={{ color: 'var(--color-primary)', fontWeight: 600 }}>Ir a iniciar sesión</Link>
          </p>
        </div>
      </AuthLayout>
    );
  }

  if (success) {
    return (
      <AuthLayout>
        <div className="card" style={cardStyle}>
          <h1 style={{ fontSize: '22px', fontWeight: 700, color: 'var(--color-heading)', textAlign: 'center', marginTop: 0 }}>
            ¡Contraseña creada!
          </h1>
          <p style={{ color: 'var(--color-secondary)', textAlign: 'center' }}>
            Te llevamos a iniciar sesión...
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
            ¡Hola, {invite.fullName}!
          </h1>
          <p style={{ fontSize: '14px', color: 'var(--color-secondary)', marginTop: '6px' }}>
            {invite.doctorName} te invitó a acceder como personal del consultorio en ProPatient Clinic. Crea tu contraseña para continuar.
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

          <div>
            <label style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: 'var(--color-heading)', marginBottom: '6px' }}>
              Nueva contraseña
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={6}
              style={{ width: '100%', padding: '10px 14px', borderRadius: '8px', border: '1px solid var(--border)', fontSize: '14px', boxSizing: 'border-box' }}
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: 'var(--color-heading)', marginBottom: '6px' }}>
              Confirmar contraseña
            </label>
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              minLength={6}
              style={{ width: '100%', padding: '10px 14px', borderRadius: '8px', border: '1px solid var(--border)', fontSize: '14px', boxSizing: 'border-box' }}
            />
          </div>

          <button type="submit" className="btn-primary login-button" disabled={submitting}>
            {submitting ? 'Creando...' : 'Crear contraseña'}
          </button>
        </form>
      </div>
    </AuthLayout>
  );
};
