import React, { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import axios from 'axios';
import api from '../api/axios';
import { formatToLocalDate } from '../utils/dateFormatter';
import { sanitizePhoneInput } from '../utils/phoneInput';
import { ConfirmDialog } from '../components/ConfirmDialog';
import './PatientForm.scss';

interface DuplicatePatient {
  id: number;
  firstName: string;
  lastName: string;
}

export const PatientForm: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const isEdit = !!id;
  const [loading, setLoading] = useState(false);
  const [duplicatePatient, setDuplicatePatient] = useState<DuplicatePatient | null>(null);
  const [formData, setFormData] = useState({
    firstName: '',
    lastName: '',
    email: '',
    phone: '',
    birthDate: '',
    gender: 'M'
  });

  useEffect(() => {
    if (isEdit) {
      const fetchPatient = async () => {
        try {
          const response = await api.get(`/patients/${id}`);
          const p = response.data;
          setFormData({
            firstName: p.firstName || '',
            lastName: p.lastName || '',
            email: p.email || '',
            phone: p.phone || '',
            birthDate: p.birthDate ? p.birthDate.split('T')[0] : '',
            gender: p.gender || 'M'
          });
        } catch (error) {
          console.error("Error al cargar datos del paciente:", error);
        }
      };
      fetchPatient();
    }
  }, [id, isEdit]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Validar formato de correo si se ha ingresado uno (ya que es opcional)
    if (formData.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
      alert("Por favor, ingrese un correo electrónico válido.");
      return;
    }

    setLoading(true);
    try {
      if (isEdit) {
        await api.put(`/patients/${id}`, formData);
      } else {
        await api.post('/patients', formData);
      }
      navigate('/pacientes');
    } catch (error) {
      if (axios.isAxiosError(error) && error.response?.status === 409 && error.response.data?.error === 'patient_exists') {
        setDuplicatePatient(error.response.data.patient);
      } else {
        alert("Error al guardar la información del paciente.");
      }
    } finally {
      setLoading(false);
    }
  };

  const handleConfirmLink = async () => {
    if (!duplicatePatient) return;
    setLoading(true);
    try {
      await api.post(`/patients/${duplicatePatient.id}/link`);
      navigate(`/pacientes/${duplicatePatient.id}`);
    } catch (error) {
      alert("No se pudo vincular al paciente existente.");
    } finally {
      setLoading(false);
      setDuplicatePatient(null);
    }
  };

  return (
    <div className="patient-form-container">
      <header className="page-header">
        <h1>{isEdit ? 'Editar Perfil del Paciente' : 'Registrar Nuevo Paciente'}</h1>
        <button className="btn-text" onClick={() => navigate(-1)}>Cancelar</button>
      </header>

      <form onSubmit={handleSubmit} className="card form-card">
        <div className="form-row">
          <div className="form-group">
            <label>Nombre(s)</label>
            <input 
              type="text" 
              value={formData.firstName} 
              onChange={e => setFormData({...formData, firstName: e.target.value})} 
              required 
            />
          </div>
          <div className="form-group">
            <label>Apellido(s)</label>
            <input 
              type="text" 
              value={formData.lastName} 
              onChange={e => setFormData({...formData, lastName: e.target.value})} 
              required 
            />
          </div>
        </div>

        <div className="form-group">
          <label>Correo Electrónico (Opcional)</label>
          <input 
            type="email" 
            value={formData.email} 
            onChange={e => setFormData({...formData, email: e.target.value})} 
          />
        </div>

        <div className="form-row">
          <div className="form-group">
            <label>Teléfono</label>
            <input
              type="tel"
              value={formData.phone}
              onChange={e => setFormData({...formData, phone: sanitizePhoneInput(e.target.value)})}
              required
            />
          </div>
          <div className="form-group">
            <label>Fecha de Nacimiento</label>
            <input 
              type="date" 
              value={formData.birthDate} 
              onChange={e => setFormData({...formData, birthDate: e.target.value})} 
            />
          </div>
        </div>

        <div className="form-group">
          <label>Género</label>
          <select 
            name="gender" 
            value={formData.gender} 
            onChange={e => setFormData({...formData, gender: e.target.value})}
          >
            <option value="M">Masculino</option>
            <option value="F">Femenino</option>
            <option value="O">Otro</option>
          </select>
        </div>

        <button type="submit" className="btn-primary btn-block" disabled={loading}>
          {loading ? 'Guardando...' : 'Guardar Paciente'}
        </button>
      </form>

      <ConfirmDialog
        isOpen={!!duplicatePatient}
        title="Paciente ya registrado"
        message={`Ya existe un paciente registrado con este correo: ${duplicatePatient?.firstName || ''} ${duplicatePatient?.lastName || ''}. ¿Deseas vincularlo a tu lista (podrás ver sus antecedentes) en vez de crear uno nuevo?`}
        confirmText="Sí, vincular"
        cancelText="Cancelar"
        onConfirm={handleConfirmLink}
        onCancel={() => setDuplicatePatient(null)}
      />
    </div>
  );
};