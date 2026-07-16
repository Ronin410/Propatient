import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout';
import { getErrorMessage } from '../utils/errorMessage';
import './Login.scss';

export const StaffLogin: React.FC = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { loginStaff } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);
    try {
      const res = await api.post('/auth/staff-login', { email, password });
      loginStaff(res.data.token, res.data.doctorName || '');
      navigate('/inicio');
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo iniciar sesión. Intenta de nuevo.'));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <AuthLayout>
      <div
        className="card"
        style={{
          padding: '48px 35px 40px 35px',
          backgroundColor: 'var(--bg)',
          borderRadius: '24px',
          boxShadow: '0 20px 40px rgba(0, 0, 0, 0.04), 0 1px 3px rgba(0, 0, 0, 0.02)',
          boxSizing: 'border-box',
          width: '100%'
        }}
      >
        <div style={{ textAlign: 'center', marginBottom: '32px' }}>
          <h1 style={{ fontSize: '26px', fontWeight: 700, color: 'var(--color-heading)', margin: 0, letterSpacing: '-0.5px' }}>
            Acceso de Personal
          </h1>
          <p style={{ fontSize: '14px', color: 'var(--color-secondary)', marginTop: '6px', fontWeight: 500 }}>
            ProPatient Medical System
          </p>
        </div>

        <form className="login-form" onSubmit={handleSubmit}>
          {error && (
            <div
              className="alert alert-danger"
              style={{
                marginBottom: '1rem',
                padding: '12px 14px',
                borderRadius: '8px',
                backgroundColor: 'var(--color-danger-bg)',
                color: 'var(--color-danger)',
                fontSize: '13px',
                border: '1px solid var(--color-danger-border)'
              }}
            >
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
              style={{
                width: '100%', padding: '10px 14px', borderRadius: '8px',
                border: '1px solid var(--border)', fontSize: '14px', boxSizing: 'border-box'
              }}
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '13px', fontWeight: 600, color: 'var(--color-heading)', marginBottom: '6px' }}>
              Contraseña
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              style={{
                width: '100%', padding: '10px 14px', borderRadius: '8px',
                border: '1px solid var(--border)', fontSize: '14px', boxSizing: 'border-box'
              }}
            />
          </div>

          <button type="submit" className="btn-primary login-button" disabled={isLoading}>
            {isLoading ? 'Ingresando...' : 'Iniciar sesión'}
          </button>

          <p style={{ fontSize: '13px', color: 'var(--color-secondary)', textAlign: 'center', marginTop: '4px' }}>
            <Link to="/personal/recuperar" style={{ color: 'var(--color-primary)', fontWeight: 600 }}>¿Olvidaste tu contraseña?</Link>
          </p>

          <p style={{ fontSize: '13px', color: 'var(--color-secondary)', textAlign: 'center', marginTop: '8px' }}>
            ¿Eres el doctor dueño del consultorio? <Link to="/login" style={{ color: 'var(--color-primary)', fontWeight: 600 }}>Inicia sesión aquí</Link>
          </p>
        </form>
      </div>
    </AuthLayout>
  );
};
