import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
// Importamos la utilidad de fechas según las reglas del Prompt
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
      // Usamos la instancia de Axios que ya tiene la URL base y el Token
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

  // Carga inicial
  useEffect(() => {
    fetchPatients();
  }, []);

  // Búsqueda con debounce simple
  useEffect(() => {
    const delayDebounceFn = setTimeout(() => {
      if (searchTerm.length > 2 || searchTerm.length === 0) {
        fetchPatients(searchTerm);
      }
    }, 500);

    return () => clearTimeout(delayDebounceFn);
  }, [searchTerm]);

  return (
    <div className="patients-container">
      <header className="page-header">
        <div>
          <h1>Mis Pacientes</h1>
          <p className="subtitle">Gestiona la base de datos de personas vinculadas a tu consulta.</p>
        </div>
        <button className="btn-primary" onClick={() => navigate('/pacientes/nuevo')}>
          + Nuevo Paciente
        </button>
      </header>

      <div className="search-bar-container card">
        <input
          type="text"
          placeholder="Buscar por nombre, apellido o teléfono..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="search-input"
        />
      </div>

      {loading ? (
        <div className="loading-state">Cargando pacientes...</div>
      ) : (
        <div className="table-wrapper card">
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
                patients.map((patient) => (
                  <tr key={patient.id}>
                    <td className="patient-name">
                      {patient.firstName} {patient.lastName}
                    </td>
                    <td>{patient.phone || 'N/A'}</td>
                    <td>{patient.email}</td>
                    <td>
                      {/* Usamos el formateador para evitar el crash del split y manejar UTC */}
                      {patient.updated_at ? formatToLocalDate(patient.updated_at) : 'N/A'}
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
                ))
              ) : (
                <tr>
                  <td colSpan={5} className="empty-table">No se encontraron pacientes vinculados.</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};