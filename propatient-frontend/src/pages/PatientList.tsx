import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalDate } from '../utils/dateFormatter';
import type { Patient } from '../types';
import './PatientList.scss';

export const PatientList: React.FC = () => {
  const [patients, setPatients] = useState<Patient[]>([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  const fetchPatients = async (query: string = '') => {
    setLoading(true);
    try {
      const response = await api.get(query 
        ? `/patients/search?query=${query}`
        : `/patients`);
      if (response.data) {
        setPatients(response.data);
      }
    } catch (error) {
      console.error("Error cargando pacientes:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPatients();
  }, []);

  useEffect(() => {
    const delayDebounceFn = setTimeout(() => {
      if (searchTerm.length > 2 || searchTerm.length === 0) {
        fetchPatients(searchTerm);
      }
    }, 500);

    return () => clearTimeout(delayDebounceFn);
  }, [searchTerm]);

  // Obtener la inicial para el contenedor circular (Avatar)
  const getAvatarLetter = (firstName: string, lastName: string) => {
    if (firstName) return firstName.charAt(0).toUpperCase();
    if (lastName) return lastName.charAt(0).toUpperCase();
    return 'P';
  };

  return (
    <div className="patients-container">
      <header className="page-header">
        <div>
          <h1>Mis Pacientes</h1>
          <p className="subtitle">Gestiona la base de datos de personas vinculadas a tu consulta.</p>
        </div>
        <button className="btn-submit" onClick={() => navigate('/pacientes/nuevo')}>
          <span className="material-icons-outlined">person_add</span>
          Nuevo Paciente
        </button>
      </header>

      {loading ? (
        <div className="loading-state">
          <p>Cargando pacientes...</p>
        </div>
      ) : (
        <div className="card-layout">
          {/* BARRA DE BÚSQUEDA INTEGRADA */}
          <div className="search-bar-container">
            <div className="search-wrapper">
              <span className="material-icons-outlined search-icon">search</span>
              <input
                type="text"
                placeholder="Buscar por nombre, apellido o teléfono..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="search-input"
              />
            </div>
          </div>

          {/* TABLA DE PACIENTES */}
          <div className="table-wrapper">
            <table className="patients-table">
              <thead>
                <tr>
                  <th>Nombre Completo</th>
                  <th>Teléfono</th>
                  <th>Correo Electrónico</th>
                  <th>Última Actividad</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {patients.length > 0 ? (
                  patients.map((patient) => {
                    const initial = getAvatarLetter(patient.firstName, patient.lastName);
                    return (
                      <tr key={patient.id}>
                        <td>
                          <div className="patient-profile-cell">
                            <div className="avatar-patient">{initial}</div>
                            <div className="patient-info-meta">
                              <span className="patient-name">
                                {patient.firstName} {patient.lastName}
                              </span>
                            </div>
                          </div>
                        </td>
                        <td>
                          <span className="phone-text">{patient.phone || '—'}</span>
                        </td>
                        <td>
                          <span className="email-text">{patient.email || '—'}</span>
                        </td>
                        <td>
                          <span className="date-badge">
                            {patient.updated_at ? formatToLocalDate(patient.updated_at) : 'Sin registro'}
                          </span>
                        </td>
                        <td>
                          <button 
                            className="btn-outline-sm"
                            onClick={() => navigate(`/pacientes/${patient.id}`)}
                          >
                            Ver Historial
                          </button>
                        </td>
                      </tr>
                    );
                  })
                ) : (
                  <tr>
                    <td colSpan={5}>
                      <div className="empty-table">
                        <span className="material-icons-outlined">group_off</span>
                        <p>No se encontraron pacientes vinculados a tu consulta.</p>
                      </div>
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};