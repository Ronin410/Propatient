import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout';
import { getErrorMessage } from '../utils/errorMessage';
import { sanitizePhoneInput } from '../utils/phoneInput';
import { LocationPicker } from '../components/LocationPicker';
import { useAuth } from '../context/AuthContext';

export const CompleteProfile = () => {
  const [formData, setFormData] = useState({
    fullName: '',
    phone: '',
    birthDate: '',
    address: '',
    postalCode: '',
    medicalSpecialty: '',
    university: '',
    referralCode: ''
  });
  const [location, setLocation] = useState<{ latitude: number | null; longitude: number | null }>({
    latitude: null,
    longitude: null
  });
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [referralNotice, setReferralNotice] = useState<{ type: 'success' | 'info'; text: string } | null>(null);
  const navigate = useNavigate();
  const { setDoctorName } = useAuth();

  useEffect(() => {
    const googleName = localStorage.getItem('suggested_fullname');
    if (googleName) {
      setFormData((prev) => ({ ...prev, fullName: googleName }));
      localStorage.removeItem('suggested_fullname');
    }
  }, []);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData({ ...formData, [name]: name === 'phone' ? sanitizePhoneInput(value) : value });
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
      const res = await api.post('/user/update-profile', {
        ...formData,
        latitude: location.latitude != null ? String(location.latitude) : '',
        longitude: location.longitude != null ? String(location.longitude) : ''
      });
      setDoctorName(formData.fullName);

      // Confirmación (o aviso neutro) del código de invitación, sin
      // bloquear nunca el avance al siguiente paso — ver
      // referral.ApplyCodeIfValid en el backend.
      if (formData.referralCode.trim()) {
        if (res.data.referralCodeApplied) {
          setReferralNotice({ type: 'success', text: '¡Código de invitación aplicado! Ganarás una semana gratis cuando actives tu suscripción.' });
        } else {
          setReferralNotice({ type: 'info', text: 'Ese código de invitación no es válido o ya no está disponible.' });
        }
        window.scrollTo({ top: 0, behavior: 'smooth' });
        setTimeout(() => navigate('/registro/validar-cedula'), 1800);
      } else {
        navigate('/registro/validar-cedula');
      }
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'Ocurrió un error al guardar tu perfil.'));
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
    background: 'var(--bg)',
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
        backgroundColor: 'var(--bg)',
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
              backgroundColor: 'var(--color-danger-bg)',
              color: 'var(--color-danger)',
              fontSize: '14px'
            }}>
              {error}
            </div>
          )}

          {referralNotice && (
            <div style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              lineHeight: '1.4',
              padding: '12px',
              borderRadius: '6px',
              backgroundColor: referralNotice.type === 'success' ? 'var(--color-success-bg)' : 'var(--color-warning-bg)',
              color: referralNotice.type === 'success' ? 'var(--color-success)' : 'var(--color-warning)',
              fontSize: '14px'
            }}>
              {referralNotice.text}
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
              Ubicación en el Mapa <span style={{ fontWeight: 'normal', fontSize: '12px' }}>(Opcional)</span>
            </label>
            <LocationPicker
              address={formData.address}
              latitude={location.latitude}
              longitude={location.longitude}
              onChange={(lat, lng) => setLocation({ latitude: lat, longitude: lng })}
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

          <div>
            <label style={{ ...labelStyle, color: 'var(--text)' }}>
              Código de Invitación <span style={{ fontWeight: 'normal', fontSize: '12px' }}>(Opcional)</span>
            </label>
            <input
              type="text"
              name="referralCode"
              placeholder="Código de un colega que te invitó"
              style={{ ...inputStyle, textTransform: 'uppercase' }}
              value={formData.referralCode}
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