import React, { useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout';
import { getErrorMessage } from '../utils/errorMessage';
import './Login.scss';

// A diferencia de AcceptStaffInvite.tsx, no hay un GET previo para validar
// el token: el formulario se muestra directo y el error de token
// inválido/vencido se muestra recién al enviar (un paso de red menos, el
// backend igual valida todo en ResetStaffPassword).
export const ResetStaffPassword: React.FC = () => {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();

  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

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
      await api.post(`/auth/staff-password-reset/${token}`, { password });
      setSuccess(true);
      setTimeout(() => navigate('/staff-login'), 2000);
    } catch (err: unknown) {
      setFormError(getErrorMessage(err, 'No se pudo restablecer la contraseña. Solicita un nuevo link.'));
    } finally {
      setSubmitting(false);
    }
  };

  if (success) {
    return (
      <AuthLayout>
        <div className="card" style={cardStyle}>
          <h1 style={{ fontSize: '22px', fontWeight: 700, color: 'var(--color-heading)', textAlign: 'center', marginTop: 0 }}>
            ¡Contraseña actualizada!
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
            Restablecer contraseña
          </h1>
          <p style={{ fontSize: '14px', color: 'var(--color-secondary)', marginTop: '6px' }}>
            Escribe tu nueva contraseña de acceso.
          </p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          {formError && (
            <div style={{
              padding: '12px 14px', borderRadius: '8px',
              backgroundColor: 'var(--color-danger-bg)', color: 'var(--color-danger)',
              fontSize: '13px', border: '1px solid var(--color-danger-border)'
            }}>
              {formError}
              {formError.includes('vencido') || formError.includes('válido') ? (
                <>
                  {' '}
                  <Link to="/personal/recuperar" style={{ color: 'inherit', fontWeight: 700 }}>Pedir uno nuevo</Link>
                </>
              ) : null}
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
              autoFocus
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
            {submitting ? 'Guardando...' : 'Restablecer contraseña'}
          </button>
        </form>
      </div>
    </AuthLayout>
  );
};
