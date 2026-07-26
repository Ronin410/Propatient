import React, { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import adminApi from '../api/adminAxios';
import { getErrorMessage } from '../utils/errorMessage';
import { Popup } from '../components/Popup';
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
  // Cubre tanto la prueba real de 14 días como un acceso gratuito
  // otorgado a mano (ver GrantFreeAccess) — ambos casos llegan como
  // "trialing" con esta misma fecha. Para un doctor de clínica, ya viene
  // resuelta con la fecha DE LA CLÍNICA (ver GrantClinicFreeAccess), no
  // la propia del doctor.
  trialEndsAt: string | null;
  // Fecha de renovación de una suscripción de Stripe ya activa (pagada
  // de verdad), consultada en vivo — null si no aplica. Para un doctor de
  // clínica, la de la suscripción de la clínica.
  currentPeriodEnd: string | null;
  isClinicMember: boolean;
  isClinicOwner: boolean;
  clinicId?: number;
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

const formatDate = (iso: string) =>
  new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' });

// Redondea hacia arriba para que "vence en unas horas" cuente como 1 día
// en vez de 0 — más intuitivo para el superadmin que revisa esto una vez
// al día.
const daysRemaining = (iso: string) => Math.ceil((new Date(iso).getTime() - Date.now()) / (1000 * 60 * 60 * 24));

const daysRemainingLabel = (iso: string) => {
  const days = daysRemaining(iso);
  if (days < 0) return `Venció hace ${Math.abs(days)} día${Math.abs(days) === 1 ? '' : 's'}`;
  if (days === 0) return 'Vence hoy';
  return `${days} día${days === 1 ? '' : 's'} restante${days === 1 ? '' : 's'}`;
};

// Target de la acción "dar acceso gratis": puede ser un doctor individual
// o una clínica completa (ver GrantClinicFreeAccess en el backend) — un
// solo selector de fecha sirve para ambos casos.
type GrantTarget = { type: 'doctor' | 'clinic'; id: number } | null;

export const AdminDoctors: React.FC = () => {
  const navigate = useNavigate();
  const [doctors, setDoctors] = useState<AdminDoctor[]>([]);
  const [overview, setOverview] = useState<AdminOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Acceso gratuito manual (ver GrantFreeAccess/GrantClinicFreeAccess en
  // el backend) — solo un renglón a la vez tiene el selector de fecha
  // abierto.
  const [grantTarget, setGrantTarget] = useState<GrantTarget>(null);
  const [grantUntil, setGrantUntil] = useState('');
  const [grantSubmitting, setGrantSubmitting] = useState(false);
  const [feedback, setFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

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

  // Prellenar con la fecha ya guardada (si tenía) o con hoy + 30 días,
  // para no forzar a escribirla siempre desde cero.
  const prefillGrantDate = (trialEndsAt: string | null) => {
    const base = trialEndsAt ? new Date(trialEndsAt) : new Date();
    if (!trialEndsAt) base.setDate(base.getDate() + 30);
    setGrantUntil(base.toISOString().slice(0, 10));
  };

  const startGranting = (doc: AdminDoctor) => {
    setGrantTarget({ type: 'doctor', id: doc.id });
    prefillGrantDate(doc.trialEndsAt);
  };

  const startGrantingClinic = (doc: AdminDoctor) => {
    if (!doc.clinicId) return;
    setGrantTarget({ type: 'clinic', id: doc.clinicId });
    prefillGrantDate(doc.trialEndsAt);
  };

  const cancelGranting = () => {
    setGrantTarget(null);
    setGrantUntil('');
  };

  const handleConfirmGrant = async () => {
    if (!grantUntil || !grantTarget) return;
    setGrantSubmitting(true);
    try {
      const path = grantTarget.type === 'clinic'
        ? `/admin/clinics/${grantTarget.id}/grant-free-access`
        : `/admin/doctors/${grantTarget.id}/grant-free-access`;
      await adminApi.put(path, { until: grantUntil });
      setFeedback({ type: 'success', message: `Acceso gratuito otorgado hasta el ${grantUntil}.` });
      cancelGranting();
      fetchData();
    } catch (err: unknown) {
      setFeedback({ type: 'error', message: getErrorMessage(err, 'No se pudo otorgar el acceso gratuito.') });
    } finally {
      setGrantSubmitting(false);
    }
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
                  <th>Acceso gratuito</th>
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
                      {doc.subscriptionStatus === 'trialing' && doc.trialEndsAt && (
                        <div className="admin-sub-date">
                          Hasta {formatDate(doc.trialEndsAt)} · {daysRemainingLabel(doc.trialEndsAt)}
                        </div>
                      )}
                      {doc.subscriptionStatus === 'active' && doc.currentPeriodEnd && (
                        <div className="admin-sub-date">
                          Renueva {formatDate(doc.currentPeriodEnd)} · {daysRemainingLabel(doc.currentPeriodEnd)}
                        </div>
                      )}
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
                    <td data-label="Acceso gratuito">
                      {(() => {
                        const isGrantingThis =
                          (grantTarget?.type === 'doctor' && grantTarget.id === doc.id) ||
                          (grantTarget?.type === 'clinic' && grantTarget.id === doc.clinicId);

                        if (isGrantingThis) {
                          return (
                            <div className="admin-grant-form">
                              <input
                                type="date"
                                value={grantUntil}
                                min={new Date().toISOString().slice(0, 10)}
                                onChange={(e) => setGrantUntil(e.target.value)}
                              />
                              <div className="admin-grant-buttons">
                                <button
                                  className="admin-grant-confirm"
                                  disabled={grantSubmitting || !grantUntil}
                                  onClick={() => handleConfirmGrant()}
                                >
                                  {grantSubmitting ? 'Guardando...' : 'Confirmar'}
                                </button>
                                <button className="admin-grant-cancel" disabled={grantSubmitting} onClick={cancelGranting}>
                                  Cancelar
                                </button>
                              </div>
                            </div>
                          );
                        }

                        if (doc.isClinicMember) {
                          // Un solo botón por clínica (en el renglón del
                          // dueño) para no repetir la misma acción una vez
                          // por cada doctor de la clínica — afecta a todos
                          // por igual (ver GrantClinicFreeAccess).
                          if (!doc.isClinicOwner) {
                            return <span className="admin-grant-disabled">Lo administra el dueño de la clínica</span>;
                          }
                          return (
                            <button className="admin-grant-btn" onClick={() => startGrantingClinic(doc)}>
                              Dar acceso gratis a la clínica
                            </button>
                          );
                        }

                        return (
                          <button className="admin-grant-btn" onClick={() => startGranting(doc)}>
                            Dar acceso gratis
                          </button>
                        );
                      })()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {doctors.length === 0 && <p className="admin-pending-status">Todavía no hay doctores registrados.</p>}
          </section>
        )}
      </main>

      <Popup
        isOpen={!!feedback}
        type={feedback?.type || 'success'}
        title={feedback?.type === 'error' ? 'Error' : 'Listo'}
        message={feedback?.message || ''}
        onClose={() => setFeedback(null)}
      />
    </div>
  );
};
