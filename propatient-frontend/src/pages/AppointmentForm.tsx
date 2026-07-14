import React, { useState, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import axios from 'axios';
import api from '../api/axios';
import type { Patient } from '../types';
import { Popup } from '../components/Popup'; // Asegura la ruta correcta de tu componente genérico
import { ConfirmDialog } from '../components/ConfirmDialog';
import { getErrorMessage } from '../utils/errorMessage';
import './AppointmentForm.scss';

interface DuplicatePatient {
  id: number;
  firstName: string;
  lastName: string;
}

export const AppointmentForm: React.FC = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const patientIdParam = searchParams.get('patientId');

  const [mode, setMode] = useState<'search' | 'manual'>(patientIdParam ? 'search' : 'search');
  const [selectedPatient, setSelectedPatient] = useState<Patient | null>(null);
  
  // Campos para paciente nuevo
  const [newPatient, setNewPatient] = useState({ firstName: '', lastName: '', phone: '', email: '' });

  const [observations, setObservations] = useState('');
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);
  const [duplicatePatient, setDuplicatePatient] = useState<DuplicatePatient | null>(null);

  // Estado para la búsqueda de pacientes
  const [searchTerm, setSearchTerm] = useState('');
  const [searchResults, setSearchResults] = useState<Patient[]>([]);
  const [isSearching, setIsSearching] = useState(false);

  // Configuración del Popup Genérico
  const [popupConfig, setPopupConfig] = useState({
    isOpen: false,
    type: 'success' as 'success' | 'error',
    title: '',
    message: ''
  });

  // Calculamos la fecha y hora actual en formato local para el atributo 'min'
  const now = new Date();
  const tzOffset = now.getTimezoneOffset() * 60000;
  const minDateTime = new Date(now.getTime() - tzOffset).toISOString().slice(0, 16);
  const [dateTime, setDateTime] = useState('');

  // Carga inicial si viene un ID de paciente por URL
  useEffect(() => {
    if (patientIdParam) {
      const fetchPatient = async () => {
        try {
          const res = await api.get(`/patients/${patientIdParam}`);
          if (res.data) {
            setSelectedPatient(res.data);
            setMode('search');
          }
        } catch (err) {
          console.error("Error cargando paciente de la URL:", err);
        }
      };
      fetchPatient();
    }
  }, [patientIdParam]);

  // Búsqueda en tiempo real
  useEffect(() => {
    if (mode !== 'search' || !searchTerm.trim()) {
      setSearchResults([]);
      return;
    }
    const delayDebounce = setTimeout(async () => {
      setIsSearching(true);
      try {
        const res = await api.get(`/patients/search?query=${encodeURIComponent(searchTerm)}`);
        setSearchResults(res.data || []);
      } catch (err) {
        console.error("Error buscando pacientes:", err);
      } finally {
        setIsSearching(false);
      }
    }, 3000); // Mantenemos tus 3 segundos de debounce fijados

    return () => clearTimeout(delayDebounce);
  }, [searchTerm, mode]);

  const bookAppointment = async (patientId: number) => {
    await api.post('/appointments', {
      patientId,
      appointmentDateTime: new Date(dateTime).toISOString(), // ✨ Cambiado de dateTime a appointmentDateTime
      service: reason,                                       // ✨ Cambiado de reason a service (asumiendo que guardas el motivo aquí)
      observations: observations
    });

    setPopupConfig({
      isOpen: true,
      type: 'success',
      title: 'Cita Agendada',
      message: 'La cita médica se ha registrado exitosamente en la agenda del consultorio.'
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      let patientId = selectedPatient?.id || 0;

      // Si es modo manual, primero registramos al nuevo paciente
      if (mode === 'manual') {
        try {
          const patientRes = await api.post('/patients', newPatient);
          patientId = patientRes.data.id;
        } catch (err) {
          // El paciente ya existe en el sistema (de otro doctor): esperamos
          // a que el doctor confirme si quiere vincularlo antes de continuar.
          if (axios.isAxiosError(err) && err.response?.status === 409 && err.response.data?.error === 'patient_exists') {
            setDuplicatePatient(err.response.data.patient);
            setLoading(false);
            return;
          }
          throw err;
        }
      }

      if (!patientId) {
        setPopupConfig({
          isOpen: true,
          type: 'error',
          title: 'Falta Paciente',
          message: 'Por favor selecciona un paciente existente o registra uno nuevo.'
        });
        setLoading(false);
        return;
      }

      await bookAppointment(patientId);
    } catch (err: unknown) {
      console.error(err);
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error de Registro',
        message: getErrorMessage(err, 'No se pudo agendar la cita médica. Verifica los datos.')
      });
    } finally {
      setLoading(false);
    }
  };

  const handleConfirmLink = async () => {
    if (!duplicatePatient) return;
    setLoading(true);
    try {
      await api.post(`/patients/${duplicatePatient.id}/link`);
      await bookAppointment(duplicatePatient.id);
    } catch (err: unknown) {
      console.error(err);
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error de Registro',
        message: getErrorMessage(err, 'No se pudo completar el registro con el paciente vinculado.')
      });
    } finally {
      setLoading(false);
      setDuplicatePatient(null);
    }
  };

  const handlePopupClose = () => {
    setPopupConfig(prev => ({ ...prev, isOpen: false }));
    if (popupConfig.type === 'success') {
      navigate('/appointments'); // Redirección al calendario al terminar con éxito
    }
  };

  return (
    <div className="appointment-form-container">
      <header className="page-header">
        <div>
          <h1>Agendar Nueva Cita Médica</h1>
          <p>Asigna un espacio en tu agenda, vincula expedientes y configura el motivo de la consulta.</p>
        </div>
        <button className="btn-text-back" onClick={() => navigate('/appointments')}>
          <span className="material-icons-outlined">arrow_back</span> Regresar 
        </button>
      </header>

      <form onSubmit={handleSubmit} className="profile-form">
        
        {/* SECCIÓN 1: ASIGNACIÓN DE PACIENTE */}
        <div className="profile-card-section">
          <h2 className="section-title">Información del Paciente</h2>
          
          <div className="mode-selector">
            <button
              type="button"
              className={`tab-btn ${mode === 'search' ? 'active' : ''}`}
              onClick={() => { setMode('search'); setSelectedPatient(null); }}
            >
              <span className="material-icons-outlined">search</span> Buscar Paciente Existente
            </button>
            <button
              type="button"
              className={`tab-btn ${mode === 'manual' ? 'active' : ''}`}
              onClick={() => { setMode('manual'); setSelectedPatient(null); }}
            >
              <span className="material-icons-outlined">person_add</span> Registrar Paciente Nuevo
            </button>
          </div>

          {mode === 'search' ? (
            <div className="search-wrapper">
              {selectedPatient ? (
                <div className="selected-patient-box">
                  <div className="patient-meta">
                    <span className="material-icons-outlined">account_circle</span>
                    <div>
                      <strong>{selectedPatient.firstName} {selectedPatient.lastName}</strong>
                      <span>Tel: {selectedPatient.phone || '—'} | Email: {selectedPatient.email || '—'}</span>
                    </div>
                  </div>
                  <button type="button" className="btn-remove" onClick={() => setSelectedPatient(null)}>
                    <span className="material-icons-outlined">close</span>
                  </button>
                </div>
              ) : (
                <div className="form-group full-width relative-input">
                  <label>Buscar por Nombre, Apellido o RFC</label>
                  <div className="input-with-icon">
                    <span className="material-icons-outlined input-icon">search</span>
                    <input
                      type="text"
                      placeholder="Escribe para buscar..."
                      value={searchTerm}
                      onChange={(e) => setSearchTerm(e.target.value)}
                    />
                  </div>
                  
                  {isSearching && <div className="searching-loader">Buscando en expedientes...</div>}
                  
                  {searchResults.length > 0 && (
                    <ul className="search-results">
                      {searchResults.map((p) => (
                        <li key={p.id} onClick={() => { setSelectedPatient(p); setSearchResults([]); setSearchTerm(''); }}>
                          <span className="material-icons-outlined">assignment_ind</span>
                          <div className="result-info">
                            <span className="name">{p.firstName} {p.lastName}</span>
                            <span className="sub">{p.email || 'Sin correo'} • {p.phone || 'Sin teléfono'}</span>
                          </div>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              )}
            </div>
          ) : (
            <div className="form-grid">
              <div className="form-group">
                <label>Nombre(s)</label>
                <input
                  type="text"
                  required={mode === 'manual'}
                  value={newPatient.firstName}
                  onChange={(e) => setNewPatient({ ...newPatient, firstName: e.target.value })}
                />
              </div>
              <div className="form-group">
                <label>Apellido(s)</label>
                <input
                  type="text"
                  required={mode === 'manual'}
                  value={newPatient.lastName}
                  onChange={(e) => setNewPatient({ ...newPatient, lastName: e.target.value })}
                />
              </div>
              <div className="form-group">
                <label>Teléfono Celular</label>
                <input
                  type="tel"
                  value={newPatient.phone}
                  onChange={(e) => setNewPatient({ ...newPatient, phone: e.target.value })}
                />
              </div>
              <div className="form-group">
                <label>Correo Electrónico</label>
                <input
                  type="email"
                  value={newPatient.email}
                  onChange={(e) => setNewPatient({ ...newPatient, email: e.target.value })}
                />
              </div>
            </div>
          )}
        </div>

        {/* SECCIÓN 2: DETALLES DE LA CITA */}
        <div className="profile-card-section">
          <h2 className="section-title">Fecha, Horario y Diagnóstico Inicial</h2>
          
          <div className="form-grid">
            <div className="form-group">
              <label htmlFor="datetime">Fecha y Hora Programada</label>
              <input
                id="datetime"
                type="datetime-local"
                value={dateTime}
                onChange={(e) => setDateTime(e.target.value)}
                min={minDateTime}
                required
              />
            </div>

            <div className="form-group">
              <label htmlFor="reason">Motivo Principal de Consulta</label>
              <input
                id="reason"
                type="text"
                placeholder="Ej: Control mensual, Valoración de estudios, etc."
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                required
              />
            </div>

            <div className="form-group full-width">
              <label htmlFor="obs">Observaciones y Notas Clínicas Previas (Opcional)</label>
              <textarea
                id="obs"
                rows={4}
                placeholder="Añade antecedentes sutiles, síntomas descritos por teléfono o requerimientos especiales..."
                value={observations}
                onChange={(e) => setObservations(e.target.value)}
              />
            </div>
          </div>
        </div>

        {/* ACCIONES DE ENVÍO */}
        <div className="actions-container">
          <button type="submit" className="btn-save-profile" disabled={loading}>
            {loading ? 'Procesando Agenda...' : 'Confirmar y Agendar Cita'}
          </button>
        </div>

      </form>

      {/* POPUP DE NOTIFICACIÓN COMPARTIDO */}
      <Popup
        isOpen={popupConfig.isOpen}
        type={popupConfig.type}
        title={popupConfig.title}
        message={popupConfig.message}
        onClose={handlePopupClose}
      />

      <ConfirmDialog
        isOpen={!!duplicatePatient}
        title="Paciente ya registrado"
        message={`Ya existe un paciente registrado con este correo: ${duplicatePatient?.firstName || ''} ${duplicatePatient?.lastName || ''}. ¿Deseas vincularlo a tu lista (podrás ver sus antecedentes) y agendar la cita con él?`}
        confirmText="Sí, vincular y agendar"
        cancelText="Cancelar"
        onConfirm={handleConfirmLink}
        onCancel={() => setDuplicatePatient(null)}
      />
    </div>
  );
};