import React, { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import adminApi from '../api/adminAxios';
import { getErrorMessage } from '../utils/errorMessage';
import './AdminDoctors.scss';

interface MonthlyStats {
  total: number;
  completed: number;
  noShow: number;
  cancelled: number;
  bookedByDoctor: number;
  bookedPublic: number;
}

interface AdminDoctor {
  id: number;
  fullName: string;
  email: string;
  username: string;
  cedulaValidated: string;
  subscriptionStatus: string;
  trialEndsAt: string | null;
  isClinicMember: boolean;
  isClinicOwner: boolean;
  clinicName?: string;
  createdAt: string;
  monthlyStats: MonthlyStats;
}

interface AdminOverview {
  totalDoctors: number;
  doctorsByCedulaStatus: Record<string, number>;
  doctorsBySubscription: Record<string, number>;
  monthlyStats: MonthlyStats;
}

const cedulaLabels: Record<string, string> = {
  PENDIENTE: 'Pendiente',
  CAPTURADA: 'En revisión',
  VALIDADA: 'Validada',
};

const subscriptionLabels: Record<string, string> = {
  trialing: 'En prueba',
  active: 'Activa',
  past_due: 'Pago vencido',
  canceled: 'Cancelada',
  incomplete: 'Incompleta',
  clinic: 'En clínica',
};

const subscriptionLabel = (status: string) => subscriptionLabels[status] || status || '—';

export const AdminDoctors: React.FC = () => {
  const navigate = useNavigate();
  const [doctors, setDoctors] = useState<AdminDoctor[]>([]);
  const [overview, setOverview] = useState<AdminOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [doctorsRes, overviewRes] = await Promise.all([
        adminApi.get('/admin/doctors'),
        adminApi.get('/admin/overview'),
      ]);
      setDoctors(doctorsRes.data || []);
      setOverview(overviewRes.data);
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo cargar la información de doctores.'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleLogout = () => {
    localStorage.removeItem('admin_token');
    navigate('/admin/login');
  };

  return (
    <div className="admin-doctors-page">
      <header className="admin-pending-header">
        <h1>
          <span className="material-icons-outlined">groups</span>
          Doctores en la plataforma
        </h1>
        <nav className="admin-nav">
          <Link to="/admin/doctores" className="active">Doctores</Link>
          <Link to="/admin/pendientes">Cédulas pendientes</Link>
        </nav>
        <button className="admin-logout-btn" onClick={handleLogout}>
          <span className="material-icons-outlined">logout</span>
          Cerrar sesión
        </button>
      </header>

      <main className="admin-doctors-body">
        {loading && <p className="admin-pending-status">Cargando...</p>}
        {error && <p className="admin-pending-status admin-pending-error">{error}</p>}

        {!loading && !error && overview && (
          <>
            <section className="admin-overview-grid">
              <div className="admin-overview-card">
                <span className="overview-value">{overview.totalDoctors}</span>
                <span className="overview-label">Doctores totales</span>
              </div>
              <div className="admin-overview-card">
                <span className="overview-value">{overview.doctorsBySubscription.active || 0}</span>
                <span className="overview-label">Suscripción activa</span>
              </div>
              <div className="admin-overview-card">
                <span className="overview-value">{overview.doctorsBySubscription.trialing || 0}</span>
                <span className="overview-label">En prueba</span>
              </div>
              <div className="admin-overview-card">
                <span className="overview-value">{overview.doctorsBySubscription.clinic || 0}</span>
                <span className="overview-label">En clínica</span>
              </div>
              <div className="admin-overview-card warn">
                <span className="overview-value">
                  {(overview.doctorsBySubscription.past_due || 0) + (overview.doctorsBySubscription.canceled || 0)}
                </span>
                <span className="overview-label">Pago vencido / cancelada</span>
              </div>
            </section>

            <section className="admin-overview-grid">
              <div className="admin-overview-card">
                <span className="overview-value">{overview.monthlyStats.total}</span>
                <span className="overview-label">Citas este mes</span>
              </div>
              <div className="admin-overview-card">
                <span className="overview-value">{overview.monthlyStats.completed}</span>
                <span className="overview-label">Completadas</span>
              </div>
              <div className="admin-overview-card warn">
                <span className="overview-value">{overview.monthlyStats.noShow}</span>
                <span className="overview-label">No-show</span>
              </div>
              <div className="admin-overview-card warn">
                <span className="overview-value">{overview.monthlyStats.cancelled}</span>
                <span className="overview-label">Canceladas</span>
              </div>
              <div className="admin-overview-card">
                <span className="overview-value">
                  {overview.monthlyStats.bookedByDoctor} / {overview.monthlyStats.bookedPublic}
                </span>
                <span className="overview-label">Agendadas por doctor / público</span>
              </div>
            </section>
          </>
        )}

        {!loading && !error && (
          <section className="admin-doctors-table-wrapper">
            <table className="admin-doctors-table">
              <thead>
                <tr>
                  <th>Doctor</th>
                  <th>Cédula</th>
                  <th>Suscripción</th>
                  <th>Citas del mes</th>
                  <th>Origen (doctor / público)</th>
                </tr>
              </thead>
              <tbody>
                {doctors.map((doc) => (
                  <tr key={doc.id}>
                    <td data-label="Doctor">
                      <div className="doctor-name">{doc.fullName || doc.username}</div>
                      <div className="doctor-email">{doc.email}</div>
                      {doc.isClinicMember && (
                        <div className="doctor-clinic-tag">
                          {doc.isClinicOwner ? 'Dueño de clínica' : 'Personal de clínica'}
                          {doc.clinicName ? ` · ${doc.clinicName}` : ''}
                        </div>
                      )}
                    </td>
                    <td data-label="Cédula">
                      <span className={`status-badge cedula-${doc.cedulaValidated.toLowerCase()}`}>
                        {cedulaLabels[doc.cedulaValidated] || doc.cedulaValidated}
                      </span>
                    </td>
                    <td data-label="Suscripción">
                      <span className={`status-badge sub-${doc.subscriptionStatus}`}>
                        {subscriptionLabel(doc.subscriptionStatus)}
                      </span>
                    </td>
                    <td data-label="Citas del mes">
                      <div className="stats-total">{doc.monthlyStats.total} citas</div>
                      <div className="stats-breakdown">
                        {doc.monthlyStats.completed} completadas · {doc.monthlyStats.noShow} no-show · {doc.monthlyStats.cancelled} canceladas
                      </div>
                    </td>
                    <td data-label="Origen">
                      {doc.monthlyStats.bookedByDoctor} doctor / {doc.monthlyStats.bookedPublic} público
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {doctors.length === 0 && <p className="admin-pending-status">Todavía no hay doctores registrados.</p>}
          </section>
        )}
      </main>
    </div>
  );
};
