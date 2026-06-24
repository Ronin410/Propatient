import React, { useState } from 'react';
import { useAuth } from '../context/AuthContext';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { AuthLayout } from './AuthLayout'; // 🚀 Layout de pantalla dividida garantizado

export const ValidateLicense = () => {
  const [licenseNumber, setLicenseNumber] = useState('');
  const [rfc, setRfc] = useState('');
  const [curp, setCurp] = useState('');
  const [fee, setFee] = useState('0');
  const [ineFile, setIneFile] = useState<File | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const { logout } = useAuth();
  const navigate = useNavigate();

  // 🎯 EXPRESIONES REGULARES DE VALIDACIÓN (MÉXICO)
  const CURP_REGEX = /^([A-Z][AEIOUX][A-Z]{2}\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])[HM](AS|BC|BS|CC|CH|CH|CL|CM|DF|DG|GR|GT|HG|JC|MC|MN|MS|NT|NL|OC|PL|QT|QR|SP|SL|SR|TC|TS|TL|VZ|YN|ZS)[B-DF-HJ-NP-TV-Z]{3}[A-Z\d])(\d)$/;
  
  // Acepta Persona Física (13 letras/números) o Persona Moral (12 letras/números) con Homoclave
  const RFC_REGEX = /^([A-ZÑ&]{3,4}) ?(\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])) ?([A-Z\d]{3})$/;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // 1. Validación de Cédula (Entre 7 y 25 caracteres)
    const cleanLicense = licenseNumber.trim();
    if (cleanLicense.length < 7 || cleanLicense.length > 25) {
      setError("La cédula profesional debe tener entre 7 y 25 caracteres.");
      return;
    }

    // 2. Validación de CURP (Solo si el usuario escribió algo)
    if (curp.trim() !== "") {
      if (!CURP_REGEX.test(curp.trim().toUpperCase())) {
        setError("El formato de la CURP es inválido. Recuerda que debe tener 18 caracteres oficiales.");
        return;
      }
    }

    // 3. Validación de RFC (Solo si el usuario escribió algo)
    if (rfc.trim() !== "") {
      if (!RFC_REGEX.test(rfc.trim().toUpperCase())) {
        setError("El formato del RFC es inválido. Debe contener homoclave (12 caracteres para empresas/entidades o 13 para médicos físicos).");
        return;
      }
    }

    // 4. Archivos obligatorios
    if (!ineFile) {
      setError("La foto o archivo de tu identificación oficial (INE / Pasaporte) es obligatoria.");
      return;
    }

    setIsLoading(true);

    try {
      const formData = new FormData();
      formData.append('licenseNumber', cleanLicense);
      formData.append('rfc', rfc.trim().toUpperCase());
      formData.append('curp', curp.trim().toUpperCase());
      formData.append('baseConsultationFee', fee);
      formData.append('ineDocument', ineFile);

      await api.post('/user/update-license-full', formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
      });

      alert("¡Documentación e información recibida con éxito!\n\nTu cuenta y cédula profesional han entrado en proceso de validación por nuestro equipo técnico. Te notificaremos por correo una vez concluido el proceso.");
      
      logout();
      navigate('/login');

    } catch (err: any) {
      setError(err.response?.data?.error || "Hubo un error al subir tus datos. Inténtalo de nuevo.");
      window.scrollTo({ top: 0, behavior: 'smooth' });
    } finally {
      setIsLoading(false);
    }
  };

  const inputStyle: React.CSSProperties = {
    width: '100%',
    padding: '10px 14px',
    borderRadius: '8px',
    border: '1px solid var(--border, #ccc)',
    fontSize: '15px',
    color: 'var(--text-h, #333)',
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
    color: 'var(--text-h, #333)'
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
        <h2 style={{ textAlign: 'center', fontWeight: 700, marginBottom: '8px', marginTop: 0, color: 'var(--text-h, #333)' }}>
          Paso 2: Validación Profesional e Identidad
        </h2>
        <p style={{ color: 'var(--text, #666)', fontSize: '14px', textAlign: 'center', marginBottom: '28px', marginTop: 0 }}>
          Completa tus datos profesionales y adjunta tu identificación oficial para iniciar la verificación de tu consultorio.
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
              ⚠️ {error}
            </div>
          )}

          <div>
            <label style={labelStyle}>Número de Cédula Profesional *</label>
            <input 
              type="text" 
              placeholder="Mínimo 7, máximo 25 caracteres" 
              style={inputStyle} 
              value={licenseNumber} 
              onChange={(e) => setLicenseNumber(e.target.value)} 
              required 
            />
          </div>

          <div style={{ display: 'flex', gap: '15px', flexWrap: 'wrap' }}>
            <div style={{ flex: '1 1 200px' }}>
              <label style={labelStyle}>CURP</label>
              <input 
                type="text" 
                placeholder="18 caracteres oficiales" 
                style={inputStyle} 
                maxLength={18}
                value={curp} 
                onChange={(e) => setCurp(e.target.value.toUpperCase())} 
              />
            </div>
            
            <div style={{ flex: '1 1 200px' }}>
              <label style={labelStyle}>RFC (Médico o Entidad)</label>
              <input 
                type="text" 
                placeholder="12 o 13 caracteres con homoclave" 
                style={inputStyle} 
                maxLength={13}
                value={rfc} 
                onChange={(e) => setRfc(e.target.value.toUpperCase())} 
              />
            </div>
          </div>

          <div>
            <label style={labelStyle}>Costo Base de Consulta ($ MXN)</label>
            <input 
              type="number" 
              min="0" 
              style={inputStyle} 
              value={fee} 
              onChange={(e) => setFee(e.target.value)} 
            />
          </div>

          <div style={{ 
            background: '#f8f9fa', 
            padding: '20px', 
            borderRadius: '8px', 
            border: '2px dashed var(--border, #ccc)',
            textAlign: 'center'
          }}>
            <label style={{ ...labelStyle, marginBottom: '10px' }}>
              Adjuntar Identificación Oficial (INE / Pasaporte) *
            </label>
            <input 
              type="file" 
              accept="image/*,application/pdf" 
              onChange={(e) => setIneFile(e.target.files?.[0] || null)} 
              style={{ fontSize: '14px', maxWidth: '100%' }}
              required 
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
            {isLoading ? "Subiendo expedientes..." : "Enviar y Finalizar Registro"}
          </button>
        </form>
      </div>
    </AuthLayout>
  );
};