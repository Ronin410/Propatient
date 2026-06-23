import React, { useState } from 'react';
import { useAuth } from '../context/AuthContext';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';

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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!licenseNumber || !ineFile) {
      setError("El número de cédula y la foto de tu INE son obligatorios.");
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      // 🚀 Envolvemos campos y archivo en FormData
      const formData = new FormData();
      formData.append('licenseNumber', licenseNumber);
      formData.append('rfc', rfc);
      formData.append('curp', curp);
      formData.append('baseConsultationFee', fee);
      formData.append('ineDocument', ineFile); // El archivo físico

      // Enviamos el bloque completo en una sola petición POST o PUT
      await api.post('/user/update-license-full', formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
      });

      // 🔔 Mensaje que solicitas de "En proceso de validación"
      alert("¡Documentación e información recibida con éxito!\n\nTu cuenta y cédula profesional han entrado en proceso de validación por nuestro equipo técnico. Te notificaremos por correo una vez concluido el proceso.");
      
      // Cerramos sesión por seguridad para que no intente saltarse el flujo usando el historial
      logout();
      navigate('/login');

    } catch (err: any) {
      setError(err.response?.data?.error || "Hubo un error al subir tus datos. Inténtalo de nuevo.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', backgroundColor: '#f5f7fb', padding: '20px' }}>
      <div style={{ background: '#fff', padding: '30px', borderRadius: '8px', boxShadow: '0 4px 12px rgba(0,0,0,0.1)', width: '100%', maxWidth: '500px' }}>
        <h2 style={{ textAlign: 'center', marginBottom: '10px', color: '#333' }}>Paso 2: Validación Profesional e Identidad</h2>
        <p style={{ fontSize: '13px', color: '#666', textAlign: 'center', marginBottom: '25px' }}>
          Completa tus datos profesionales y adjunta tu identificación oficial para iniciar la verificación de tu consultorio.
        </p>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
          {error && <div style={{ color: '#a00', background: '#fdd', padding: '10px', borderRadius: '4px', fontSize: '14px' }}>⚠️ {error}</div>}

          <div>
            <label style={{ display: 'block', fontSize: '14px', marginBottom: '5px', fontWeight: 'bold' }}>Número de Cédula Profesional *</label>
            <input type="text" placeholder="Ingresa tus 7 u 8 dígitos" style={{ width: '100%', padding: '10px', boxSizing: 'border-box' }} value={licenseNumber} onChange={(e) => setLicenseNumber(e.target.value)} required />
          </div>

          <div style={{ display: 'flex', gap: '15px' }}>
            <div style={{ flex: 1 }}>
              <label style={{ display: 'block', fontSize: '14px', marginBottom: '5px', fontWeight: 'bold' }}>CURP</label>
              <input type="text" placeholder="18 caracteres" style={{ width: '100%', padding: '10px', boxSizing: 'border-box' }} value={curp} onChange={(e) => setCurp(e.target.value.toUpperCase())} />
            </div>
            <div style={{ flex: 1 }}>
              <label style={{ display: 'block', fontSize: '14px', marginBottom: '5px', fontWeight: 'bold' }}>RFC</label>
              <input type="text" placeholder="Con homoclave" style={{ width: '100%', padding: '10px', boxSizing: 'border-box' }} value={rfc} onChange={(e) => setRfc(e.target.value.toUpperCase())} />
            </div>
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '14px', marginBottom: '5px', fontWeight: 'bold' }}>Costo Base de Consulta ($ MXN)</label>
            <input type="number" min="0" style={{ width: '100%', padding: '10px', boxSizing: 'border-box' }} value={fee} onChange={(e) => setFee(e.target.value)} />
          </div>

          <div style={{ background: '#f8f9fa', padding: '15px', borderRadius: '6px', border: '1px dashed #ccc' }}>
            <label style={{ display: 'block', fontSize: '14px', marginBottom: '8px', fontWeight: 'bold' }}>Adjuntar Identificación Oficial (INE / Pasaporte) *</label>
            <input type="file" accept="image/*,application/pdf" onChange={(e) => setIneFile(e.target.files?.[0] || null)} required />
            <span style={{ fontSize: '11px', color: '#888', display: 'block', marginTop: '5px' }}>Formatos aceptados: JPG, PNG o PDF. Máx 5MB.</span>
          </div>

          <button type="submit" disabled={isLoading} style={{ width: '100%', padding: '12px', background: '#28a745', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '16px', fontWeight: 'bold', marginTop: '10px' }}>
            {isLoading ? "Subiendo expedientes..." : "Enviar y Finalizar Registro"}
          </button>
        </form>
      </div>
    </div>
  );
};