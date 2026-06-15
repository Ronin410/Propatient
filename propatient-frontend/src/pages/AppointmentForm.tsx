import React, { useState, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import api from '../api/axios';
import type { Patient } from '../types';
import './AppointmentForm.scss';

export const AppointmentForm: React.FC = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const patientIdParam = searchParams.get('patientId');

  const [mode, setMode] = useState<'search' | 'manual'>(patientIdParam ? 'search' : 'search');
  const [selectedPatient, setSelectedPatient] = useState<Patient | null>(null);
  
  // Campos para paciente nuevo
  const [newPatient, setNewPatient] = useState({ firstName: '', lastName: '', phone: '', email: '' });

  const [dateTime, setDateTime] = useState('');
  const [reason, setReason] = useState('');
  const [observations, setObservations] = useState('');
  const [loading, setLoading] = useState(false);

  // Estado para la búsqueda de pacientes
  const [searchTerm, setSearchTerm] = useState('');
  const [searchResults, setSearchResults] = useState<Patient[]>([]);
  const [isSearching, setIsSearching] = useState(false);

  // Calculamos la fecha y hora actual en formato local para el atributo 'min'
  const now = new Date();
  const tzOffset = now.getTimezoneOffset() * 60000;
  const minDateTime = new Date(now.getTime() - tzOffset).toISOString().slice(0, 16);

  useEffect(() => {
    if (patientIdParam) {
      const fetchPatient = async () => {
        try {
          const response = await api.get(`/patients/${patientIdParam}`);
          setSelectedPatient(response.data);
        } catch (error) {
          console.error("Error al cargar paciente pre-seleccionado:", error);
        }
      };
      fetchPatient();
    }
  }, [patientIdParam]);

  const searchPatients = async (query: string) => {
    setSearchTerm(query);
    if (query.length > 2) {
      setIsSearching(true);
      try {
        const response = await api.get(`/patients/search?query=${query}`);
        setSearchResults(response.data || []);
      } catch (error) {
        console.error("Error buscando pacientes:", error);
      } finally {
        setIsSearching(false);
      }
    } else {
      setSearchResults([]);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (mode === 'search' && !selectedPatient) {
      alert("Por favor selecciona un paciente.");
      return;
    }

    if (mode === 'manual' && newPatient.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(newPatient.email)) {
      alert("El formato del correo electrónico para el nuevo paciente no es válido.");
      return;
    }

    setLoading(true);
    try {
      // REGLA CRÍTICA: Convertimos la fecha local del input a UTC antes de enviar
      const localDate = new Date(dateTime);
      const utcDateTime = localDate.toISOString();

      const patientId = selectedPatient?.id || (selectedPatient as any)?.ID;

      const payload = {
        appointmentDateTime: utcDateTime,
        service: reason, // El backend Java mapea 'service' al campo 'reason'
        notes: observations, // El backend Java mapea 'notes' al campo 'notes'
        registrationStatus: mode === 'search' ? 'REGISTERED' : 'PENDING_RECORD',
        ...(mode === 'search' 
          ? { patientId: patientId } 
          : { 
              patientName: `${newPatient.firstName} ${newPatient.lastName}`, 
              patientPhone: newPatient.phone 
            }
        )
      };

      await api.post('/appointments', payload);

      // Redirigir al calendario tras éxito
      navigate('/calendar');
    } catch (error) {
      console.error("Error al crear la cita:", error);
      alert("Hubo un error al agendar la cita.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="appointment-form-container">
      <header className="page-header">
        <h1>Agendar Nueva Cita</h1>
        <button className="btn-outline-sm" onClick={() => navigate(-1)}>Cancelar</button>
      </header>

      <form onSubmit={handleSubmit} className="card form-card">
        <div className="mode-selector">
          <button 
            type="button" 
            className={`tab-btn ${mode === 'search' ? 'active' : ''}`} 
            onClick={() => setMode('search')}
          >Paciente Registrado</button>
          <button 
            type="button" 
            className={`tab-btn ${mode === 'manual' ? 'active' : ''}`} 
            onClick={() => setMode('manual')}
          >Paciente Nuevo (Rápido)</button>
        </div>

        <div className="form-group">
          {mode === 'search' ? (
            selectedPatient ? (
            <div className="selected-patient-box">
              <span><strong>{selectedPatient.firstName} {selectedPatient.lastName}</strong></span>
              <button type="button" className="btn-text-danger" onClick={() => setSelectedPatient(null)}>Cambiar</button>
            </div>
          ) : (
            <div className="search-wrapper">
              <input
                type="text"
                placeholder="Buscar paciente por nombre o teléfono..."
                value={searchTerm}
                onChange={(e) => searchPatients(e.target.value)}
                className="search-input"
              />
              {searchResults.length > 0 && (
                <ul className="search-results">
                  {searchResults.map(p => (
                    <li key={p.id} onClick={() => setSelectedPatient(p)}>
                      {p.firstName} {p.lastName} <small>({p.phone})</small>
                    </li>
                  ))}
                </ul>
              )}
              {isSearching && <div className="searching-msg">Buscando...</div>}
            </div>
          )) : (
            <div className="manual-patient-fields">
              <div className="row">
                <input 
                  type="text" 
                  placeholder="Nombre(s)" 
                  value={newPatient.firstName}
                  onChange={(e) => setNewPatient({...newPatient, firstName: e.target.value})}
                  required 
                />
                <input 
                  type="text" 
                  placeholder="Apellido(s)" 
                  value={newPatient.lastName}
                  onChange={(e) => setNewPatient({...newPatient, lastName: e.target.value})}
                  required 
                />
              </div>
              <div className="row mt-2">
                <input 
                  type="tel" 
                  placeholder="Teléfono" 
                  value={newPatient.phone}
                  onChange={(e) => setNewPatient({...newPatient, phone: e.target.value})}
                />
                <input 
                  type="email" 
                  placeholder="Correo (Opcional)" 
                  value={newPatient.email}
                  onChange={(e) => setNewPatient({...newPatient, email: e.target.value})}
                />
              </div>
            </div>
          )}
        </div>

        <div className="form-group">
          <label htmlFor="datetime">Fecha y Hora</label>
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
          <label htmlFor="reason">Motivo de Consulta</label>
          <input
            id="reason"
            type="text"
            placeholder="Ej: Control mensual, Dolor abdominal..."
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            required
          />
        </div>

        <div className="form-group">
          <label htmlFor="obs">Observaciones (Opcional)</label>
          <textarea
            id="obs"
            rows={4}
            placeholder="Notas adicionales para la cita..."
            value={observations}
            onChange={(e) => setObservations(e.target.value)}
          />
        </div>

        <button type="submit" className="btn-primary btn-block" disabled={loading}>
          {loading ? 'Agendando...' : 'Agendar Cita'}
        </button>
      </form>
    </div>
  );
};