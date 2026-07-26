import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import adminApi from '../api/adminAxios';
import { getErrorMessage } from '../utils/errorMessage';
import './AdminLogin.scss';

// Login del panel interno de administración (revisión de cédula
// profesional). Deliberadamente NO comparte estilo con Login.tsx/AuthLayout
// (el login de marca de los doctores) — el aspecto neutro/oscuro es a
// propósito, para que nadie confunda esta pantalla con el producto.
export const AdminLogin: React.FC = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);
    try {
      const res = await adminApi.post('/admin/login', { username, password });
      localStorage.setItem('admin_token', res.data.token);
      navigate('/admin/pendientes');
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo iniciar sesión.'));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="admin-login-page">
      <form className="admin-login-card" onSubmit={handleSubmit}>
        <span className="material-icons-outlined admin-login-icon">admin_panel_settings</span>
        <h1>Panel de administración</h1>
        <p className="admin-login-subtitle">Acceso interno de ProPatient</p>

        {error && <div className="admin-login-error">{error}</div>}

        <label>
          Usuario
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            autoFocus
          />
        </label>

        <label>
          Contraseña
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </label>

        <button type="submit" disabled={isLoading}>
          {isLoading ? 'Ingresando...' : 'Iniciar sesión'}
        </button>
      </form>
    </div>
  );
};
