import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';

export const CompleteProfile = () => {
  const [formData, setFormData] = useState({
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

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFormData({ ...formData, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    try {
      await api.post('/user/update-profile', formData);
      navigate('/registro/validar-cedula'); // Avanza al siguiente paso de forma obligatoria
    } catch (err: any) {
      setError(err.response?.data?.error || 'Ocurrió un error al guardar tu perfil.');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="onboarding-container" style={{ maxWidth: '500px', margin: '50px auto', padding: '20px', boxShadow: '0 4px 10px rgba(0,0,0,0.1)', borderRadius: '8px', fontFamily: 'sans-serif' }}>
      <h2 style={{ textAlign: 'center', color: '#333' }}>Paso 1: Información Profesional</h2>
      <p style={{ color: '#666', fontSize: '14px', textAlign: 'center', marginBottom: '20px' }}>Completa tus datos generales para personalizar tu consultorio digital.</p>

      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '15px' }}>
        {error && <div style={{ color: '#a00', background: '#fdd', padding: '10px', borderRadius: '4px' }}>⚠️ {error}</div>}
        
        <div>
          <label style={{ display: 'block', marginBottom: '5px', fontWeight: 'bold' }}>Especialidad Médica</label>
          <input type="text" name="medicalSpecialty" placeholder="Ej. Pediatría, Ginecología" style={{ width: '100%', padding: '8px', boxSizing: 'border-box' }} value={formData.medicalSpecialty} onChange={handleChange} required />
        </div>

        <div>
          <label style={{ display: 'block', marginBottom: '5px', fontWeight: 'bold' }}>Universidad de Egreso</label>
          <input type="text" name="university" placeholder="Universidad donde estudiaste" style={{ width: '100%', padding: '8px', boxSizing: 'border-box' }} value={formData.university} onChange={handleChange} required />
        </div>

        <div>
          <label style={{ display: 'block', marginBottom: '5px', fontWeight: 'bold' }}>Teléfono Celular</label>
          <input type="tel" name="phone" placeholder="10 dígitos" style={{ width: '100%', padding: '8px', boxSizing: 'border-box' }} value={formData.phone} onChange={handleChange} required />
        </div>

        <div>
          <label style={{ display: 'block', marginBottom: '5px', fontWeight: 'bold' }}>Fecha de Nacimiento</label>
          <input type="date" name="birthDate" style={{ width: '100%', padding: '8px', boxSizing: 'border-box' }} value={formData.birthDate} onChange={handleChange} required />
        </div>

        <div>
          <label style={{ display: 'block', marginBottom: '5px', fontWeight: 'bold' }}>Dirección del Consultorio</label>
          <input type="text" name="address" placeholder="Calle, Número, Colonia" style={{ width: '100%', padding: '8px', boxSizing: 'border-box' }} value={formData.address} onChange={handleChange} required />
        </div>

        <div>
          <label style={{ display: 'block', marginBottom: '5px', fontWeight: 'bold' }}>Código Postal</label>
          <input type="text" name="postalCode" placeholder="Ej. 80000" style={{ width: '100%', padding: '8px', boxSizing: 'border-box' }} value={formData.postalCode} onChange={handleChange} required />
        </div>

        <button type="submit" disabled={isLoading} style={{ background: '#007bff', color: '#fff', border: 'none', padding: '12px', borderRadius: '4px', cursor: 'pointer', fontSize: '16px', fontWeight: 'bold' }}>
          {isLoading ? 'Guardando...' : 'Siguiente Paso'}
        </button>
      </form>
    </div>
  );
};