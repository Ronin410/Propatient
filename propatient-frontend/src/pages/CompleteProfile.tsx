import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout';

export const CompleteProfile = () => {
  const [formData, setFormData] = useState({
    fullName: '',
    phone: '',
    birthDate: '',
    address: '',
    postalCode: '',
    medicalSpecialty: '',
    university: ''
  });
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    const googleName = localStorage.getItem('suggested_fullname');
    if (googleName) {
      setFormData((prev) => ({ ...prev, fullName: googleName }));
      localStorage.removeItem('suggested_fullname');
    }
  }, []);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    if (!formData.fullName || !formData.phone || !formData.birthDate || !formData.medicalSpecialty || !formData.university) {
      setError('Por favor, completa todos los campos obligatorios (*).');
      setIsLoading(false);
      return;
    }

    const birthDateObj = new Date(formData.birthDate);
    const today = new Date();
    let age = today.getFullYear() - birthDateObj.getFullYear();
    const monthDiff = today.getMonth() - birthDateObj.getMonth();
    const dayDiff = today.getDate() - birthDateObj.getDate();

    if (monthDiff < 0 || (monthDiff === 0 && dayDiff < 0)) {
      age--;
    }

    if (age < 18) {
      setError('⚠️ Debes ser mayor de 18 años para poder registrarte como médico en la plataforma.');
      setIsLoading(false);
      window.scrollTo({ top: 0, behavior: 'smooth' });
      return;
    }

    try {
      await api.post('/user/update-profile', formData);
      navigate('/registro/validar-cedula'); 
    } catch (err: any) {
      setError(err.response?.data?.error || 'Ocurrió un error al guardar tu perfil.');
      window.scrollTo({ top: 0, behavior: 'smooth' });
    } finally {
      setIsLoading(false);
    }
  };

  // Estilos embebidos limpios y reutilizables basados en tu paleta
  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '10px 14px',
    borderRadius: '8px',
    border: '1px solid var(--border)',
    fontSize: '15px',
    color: 'var(--text-h)',
    outline: 'none',
    transition: 'border-color 0.2s, box-shadow 0.2s',
    background: '#ffffff',
    boxSizing: 'border-box'
  };

  const labelStyle: React.CSSProperties = {
    display: 'block',
    marginBottom: '6px',
    fontSize: '14px',
    fontWeight: 600,
    color: 'var(--text-h)'
  };

  return (
    <AuthLayout>
      <div className="card" style={{ 
        width: '100%', 
        padding: '35px',
        backgroundColor: '#ffffff',
        borderRadius: '12px',
        boxShadow: 'var(--shadow, 0 4px 12px rgba(0, 0, 0, 0.05))',
        boxSizing: 'border-box'
      }}>
        <h2 style={{ textAlign: 'center', fontWeight: 700, marginBottom: '8px', marginTop: 0, color: 'var(--text-h)' }}>
          Paso 1: Información Profesional
        </h2>
        <p style={{ color: 'var(--text)', fontSize: '14px', textAlign: 'center', marginBottom: '28px', marginTop: 0 }}>
          Completa tus datos generales para personalizar tu consultorio digital.
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
              backgroundColor: '#fee2e2',
              color: '#b91c1c',
              fontSize: '14px'
            }}>
              {error}
            </div>
          )}

          <div>
            <label style={labelStyle}>Nombre Completo *</label>
            <input 
              type="text" 
              name="fullName" 
              placeholder="Ej. Dr. Alejandro Bueno" 
              style={inputStyle} 
              value={formData.fullName} 
              onChange={handleChange} 
              onFocus={(e) => {
                e.target.style.borderColor = 'var(--accent)';
                e.target.style.boxShadow = '0 0 0 3px var(--accent-bg)';
              }}
              onBlur={(e) => {
                e.target.style.borderColor = 'var(--border)';
                e.target.style.boxShadow = 'none';
              }}
              required 
            />
          </div>

          <div>
            <label style={labelStyle}>Especialidad Médica *</label>
            <input 
              type="text" 
              name="medicalSpecialty" 
              placeholder="Ej. Pediatría, Ginecología" 
              style={inputStyle} 
              value={formData.medicalSpecialty} 
              onChange={handleChange} 
              onFocus={(e) => {
                e.target.style.borderColor = 'var(--accent)';
                e.target.style.boxShadow = '0 0 0 3px var(--accent-bg)';
              }}
              onBlur={(e) => {
                e.target.style.borderColor = 'var(--border)';
                e.target.style.boxShadow = 'none';
              }}
              required 
            />
          </div>

          <div>
            <label style={labelStyle}>Universidad de Egreso *</label>
            <input 
              type="text" 
              name="university" 
              placeholder="Universidad donde estudiaste" 
              style={inputStyle} 
              value={formData.university} 
              onChange={handleChange} 
              onFocus={(e) => {
                e.target.style.borderColor = 'var(--accent)';
                e.target.style.boxShadow = '0 0 0 3px var(--accent-bg)';
              }}
              onBlur={(e) => {
                e.target.style.borderColor = 'var(--border)';
                e.target.style.boxShadow = 'none';
              }}
              required 
            />
          </div>

          <div style={{ display: 'flex', gap: '15px', flexWrap: 'wrap' }}>
            <div style={{ flex: '1 1 200px' }}>
              <label style={labelStyle}>Teléfono Celular *</label>
              <input 
                type="text" 
                name="phone" 
                placeholder="10 dígitos" 
                style={inputStyle} 
                value={formData.phone} 
                onChange={handleChange} 
                onFocus={(e) => {
                  e.target.style.borderColor = 'var(--accent)';
                  e.target.style.boxShadow = '0 0 0 3px var(--accent-bg)';
                }}
                onBlur={(e) => {
                  e.target.style.borderColor = 'var(--border)';
                  e.target.style.boxShadow = 'none';
                }}
                required 
              />
            </div>

            <div style={{ flex: '1 1 200px' }}>
              <label style={labelStyle}>Fecha de Nacimiento *</label>
              <input 
                type="date" 
                name="birthDate" 
                style={inputStyle} 
                value={formData.birthDate} 
                onChange={handleChange} 
                onFocus={(e) => {
                  e.target.style.borderColor = 'var(--accent)';
                  e.target.style.boxShadow = '0 0 0 3px var(--accent-bg)';
                }}
                onBlur={(e) => {
                  e.target.style.borderColor = 'var(--border)';
                  e.target.style.boxShadow = 'none';
                }}
                required 
              />
            </div>
          </div>

          <hr style={{ border: 'none', borderTop: '1px solid var(--border)', margin: '10px 0' }} />

          <div>
            <label style={{ ...labelStyle, color: 'var(--text)' }}>
              Dirección del Consultorio <span style={{ fontWeight: 'normal', fontSize: '12px' }}>(Opcional)</span>
            </label>
            <input 
              type="text" 
              name="address" 
              placeholder="Calle, Número, Colonia" 
              style={inputStyle} 
              value={formData.address} 
              onChange={handleChange} 
              onFocus={(e) => {
                e.target.style.borderColor = 'var(--accent)';
                e.target.style.boxShadow = '0 0 0 3px var(--accent-bg)';
              }}
              onBlur={(e) => {
                e.target.style.borderColor = 'var(--border)';
                e.target.style.boxShadow = 'none';
              }}
            />
          </div>

          <div>
            <label style={{ ...labelStyle, color: 'var(--text)' }}>
              Código Postal <span style={{ fontWeight: 'normal', fontSize: '12px' }}>(Opcional)</span>
            </label>
            <input 
              type="text" 
              name="postalCode" 
              placeholder="Ej. 80000" 
              style={inputStyle} 
              value={formData.postalCode} 
              onChange={handleChange} 
              onFocus={(e) => {
                e.target.style.borderColor = 'var(--accent)';
                e.target.style.boxShadow = '0 0 0 3px var(--accent-bg)';
              }}
              onBlur={(e) => {
                e.target.style.borderColor = 'var(--border)';
                e.target.style.boxShadow = 'none';
              }}
            />
          </div>

          <button 
            type="submit" 
            className="btn-primary" 
            disabled={isLoading} 
            style={{ 
              width: '100%', 
              padding: '14px', 
              fontSize: '16px', 
              marginTop: '10px',
              borderRadius: '8px',
              border: 'none',
              cursor: isLoading ? 'not-allowed' : 'pointer'
            }}
          >
            {isLoading ? 'Guardando información...' : 'Siguiente Paso'}
          </button>
        </form>
      </div>
    </AuthLayout>
  );
};