import React, { useState } from 'react';
import { useAuth } from '../context/AuthContext';
import api from '../api/axios';
import { useNavigate } from 'react-router-dom';
import './Login.scss'; 

export const Login = () => {
  const [username, setUsername] = useState(''); // Cambiado de 'email' a 'username' para mayor claridad
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username || !password) {
      setError('Por favor, ingresa el usuario y la contraseña.');
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      // Se envía 'username' tal como lo espera tu backend en Go
      const response = await api.post('/auth/login', { 
        username: username, 
        password: password 
      });
      
      login(response.data.token);
      navigate('/inicio');
    } catch (err: any) {
      // Captura el mensaje de error del backend o muestra uno por defecto
      setError(err.response?.data?.error || 'Usuario o contraseña incorrectos');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="login-wrapper">
      <div className="card login-card">
        <div className="logo-container">
            <div className="medical-logo" style={{ background: 'var(--color-primary)', borderRadius: '50%', margin: '0 auto 10px' }}></div>
            <p className="office-name">ProPatient Medical System</p>
        </div>

        <h1 className="access-title">Acceso al Sistema</h1>

        <form className="login-form" onSubmit={handleSubmit}>
          {error && (
            <div className="alert alert-danger" style={{ marginBottom: '1rem', padding: '10px', borderRadius: '4px', backgroundColor: '#fdd', color: '#a00', display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span>⚠️</span>
              {error}
            </div>
          )}
          
          {/* CRÍTICO: Se cambia type="email" por type="text" para evitar la validación forzosa del "@" */}
          <input 
            type="text" 
            placeholder="Usuario o Email" 
            className="form-control"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required 
          />
          
          <input 
            type="password" 
            placeholder="Contraseña" 
            className="form-control"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required 
          />

          <button 
            type="submit" 
            className="btn btn-primary login-button" 
            disabled={isLoading}
          >
            {isLoading ? (
              <><span className="spinner-border"></span> Iniciando sesión...</>
            ) : (
              'Iniciar Sesión'
            )}
          </button>

          <a href="#" className="forgot-password-link">¿Olvidó su contraseña?</a>
        </form>
      </div>
    </div>
  );
};