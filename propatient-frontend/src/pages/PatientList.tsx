import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/axios';
import { formatToLocalDate } from '../utils/dateFormatter';
import type { Patient } from '../types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { useAuth } from '../context/AuthContext';
import './PatientList.scss';

const PAGE_SIZE = 20;

export const PatientList: React.FC = () => {
  const [patients, setPatients] = useState<Patient[]>([]);
  const [searchTerm, setSearchTerm] = useState('');
  // initialLoading: solo la primerísima carga, antes de tener nada que
  // mostrar — ahí sí tiene sentido reemplazar toda la pantalla.
  // tableLoading: cualquier carga posterior (buscar, cambiar de página) —
  // NUNCA debe desmontar la barra de búsqueda ni la tabla, solo atenuarlas
  // un poco, para no destruir y recrear el <input> (perdía el foco y se
  // sentía "rara" cada vez que se escribía algo, ver bug reportado).
  const [initialLoading, setInitialLoading] = useState(true);
  const [tableLoading, setTableLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [patientToDelete, setPatientToDelete] = useState<Patient | null>(null);
  const navigate = useNavigate();
  const { isStaff } = useAuth();

  const isSearching = searchTerm.length > 0;

  const fetchPatients = async (query: string = '', targetPage: number = 1) => {
    setTableLoading(true);
    try {
      if (query) {
        const response = await api.get(`/patients/search?query=${query}`);
        setPatients(response.data || []);
        setTotalPages(1);
      } else {
        const response = await api.get(`/patients?page=${targetPage}&pageSize=${PAGE_SIZE}`);
        setPatients(response.data?.data || []);
        setTotalPages(response.data?.totalPages || 1);
      }
    } catch (error) {
      console.error("Error cargando pacientes:", error);
    } finally {
      setTableLoading(false);
      setInitialLoading(false);
    }
  };

  useEffect(() => {
    fetchPatients('', page);
  }, [page]);

  useEffect(() => {
    const delayDebounceFn = setTimeout(() => {
      if (searchTerm.length > 2 || searchTerm.length === 0) {
        if (searchTerm.length === 0) {
          // Volver a la lista paginada normal en la página actual
          fetchPatients('', page);
        } else {
          fetchPatients(searchTerm);
        }
      }
    }, 500);

    return () => clearTimeout(delayDebounceFn);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchTerm]);

  const handleDeleteConfirmed = async () => {
    if (!patientToDelete) return;
    try {
      await api.delete(`/patients/${patientToDelete.id}`);
      setPatientToDelete(null);
      // Si era el último de la página y no es la página 1, retrocede una página
      if (patients.length === 1 && page > 1) {
        setPage((p) => p - 1);
      } else {
        fetchPatients(isSearching ? searchTerm : '', page);
      }
    } catch (error) {
      console.error("Error al eliminar paciente:", error);
      setPatientToDelete(null);
    }
  };

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

      {initialLoading ? (
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
                autoFocus
              />
            </div>
          </div>

          {/* TABLA DE PACIENTES */}
          <div className={`table-wrapper ${tableLoading ? 'is-loading' : ''}`}>
            <table className="patients-table">
              <thead>
                <tr>
                  <th>Nombre Completo</th>
                  <th>Teléfono</th>
                  <th>Correo Electrónico</th>
                  <th>Última Cita</th>
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
                        <td className="actions-cell">
                          {isStaff ? (
                            <button
                              className="btn-outline-sm"
                              onClick={() => navigate(`/pacientes/editar/${patient.id}`)}
                            >
                              Editar
                            </button>
                          ) : (
                            <button
                              className="btn-outline-sm"
                              onClick={() => navigate(`/pacientes/${patient.id}`)}
                            >
                              Ver Historial
                            </button>
                          )}
                          <button
                            className="btn-outline-sm danger"
                            onClick={() => setPatientToDelete(patient)}
                          >
                            Eliminar
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

          {!isSearching && totalPages > 1 && (
            <div className="pagination-bar">
              <button
                className="btn-outline-sm"
                disabled={page <= 1}
                onClick={() => setPage((p) => p - 1)}
              >
                Anterior
              </button>
              <span className="pagination-info">Página {page} de {totalPages}</span>
              <button
                className="btn-outline-sm"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                Siguiente
              </button>
            </div>
          )}
        </div>
      )}

      <ConfirmDialog
        isOpen={!!patientToDelete}
        variant="danger"
        title="Eliminar paciente"
        message={`¿Seguro que quieres eliminar a ${patientToDelete?.firstName || ''} ${patientToDelete?.lastName || ''} de tu lista de pacientes? Su historial clínico no se borra, solo deja de aparecer en tu consulta.`}
        confirmText="Eliminar"
        onConfirm={handleDeleteConfirmed}
        onCancel={() => setPatientToDelete(null)}
      />
    </div>
  );
};