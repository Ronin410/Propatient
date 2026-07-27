import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout';
import { getErrorMessage } from '../utils/errorMessage';
import { useAuth } from '../context/AuthContext';

// Primer paso obligatorio del onboarding, antes que el perfil o la cédula:
// el doctor debe aceptar explícitamente los Términos y Condiciones y el
// Aviso de Privacidad para poder usar el sistema. La aceptación se manda al
// backend (auth.AcceptTermsHandler), que guarda fecha/IP/versión como
// evidencia — ver models.Doctor.TermsAcceptedAt.
export const AcceptTerms = () => {
  const [checked, setChecked] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { userStatus, updateUserStatus } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!checked) {
      setError('Debes aceptar los Términos y Condiciones y el Aviso de Privacidad para continuar.');
      return;
    }
    setIsLoading(true);
    setError(null);

    try {
      await api.post('/user/accept-terms');
      updateUserStatus({ termsAccepted: true });

      if (!userStatus?.profileCompleted) {
        navigate('/registro/perfil');
      } else if (userStatus.cedulaValidated !== 'VALIDADA') {
        navigate('/registro/validar-cedula');
      } else {
        navigate('/inicio');
      }
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo registrar tu aceptación. Intenta de nuevo.'));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <AuthLayout>
      <div className="card" style={{
        width: '100%',
        padding: '35px',
        backgroundColor: 'var(--bg)',
        borderRadius: '12px',
        boxShadow: 'var(--shadow, 0 4px 12px rgba(0, 0, 0, 0.05))',
        boxSizing: 'border-box'
      }}>
        <h2 style={{ textAlign: 'center', fontWeight: 700, marginBottom: '8px', marginTop: 0, color: 'var(--text-h)' }}>
          Antes de continuar
        </h2>
        <p style={{ color: 'var(--text)', fontSize: '14px', textAlign: 'center', marginBottom: '28px', marginTop: 0 }}>
          Para usar ProPatient Clinic necesitamos que aceptes los siguientes documentos.
        </p>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
          {error && (
            <div className="alert-danger" style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              lineHeight: '1.4',
              padding: '12px',
              borderRadius: '6px',
              backgroundColor: 'var(--color-danger-bg)',
              color: 'var(--color-danger)',
              fontSize: '14px'
            }}>
              {error}
            </div>
          )}

          <label style={{ display: 'flex', alignItems: 'flex-start', gap: '10px', fontSize: '14px', color: 'var(--text)', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={checked}
              onChange={(e) => setChecked(e.target.checked)}
              style={{ marginTop: '3px', width: '16px', height: '16px', flexShrink: 0 }}
            />
            <span>
              He leído y acepto los{' '}
              <Link to="/terminos" target="_blank" rel="noopener noreferrer" style={{ color: 'var(--color-primary)', fontWeight: 600 }}>
                Términos y Condiciones
              </Link>{' '}
              y el{' '}
              <Link to="/privacidad" target="_blank" rel="noopener noreferrer" style={{ color: 'var(--color-primary)', fontWeight: 600 }}>
                Aviso de Privacidad
              </Link>{' '}
              de ProPatient Clinic.
            </span>
          </label>

          <button
            type="submit"
            className="btn-primary"
            disabled={isLoading || !checked}
            style={{
              width: '100%',
              padding: '14px',
              fontSize: '16px',
              marginTop: '10px',
              borderRadius: '8px',
              border: 'none',
              cursor: isLoading || !checked ? 'not-allowed' : 'pointer',
              opacity: !checked ? 0.6 : 1
            }}
          >
            {isLoading ? 'Guardando...' : 'Aceptar y continuar'}
          </button>
        </form>
      </div>
    </AuthLayout>
  );
};
