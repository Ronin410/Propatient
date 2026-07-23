import React, { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import adminApi from '../api/adminAxios';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { Popup } from '../components/Popup';
import { getErrorMessage } from '../utils/errorMessage';
import './AdminPendingDoctors.scss';

interface PendingDoctor {
  id: number;
  fullName: string;
  email: string;
  username: string;
  licenseNumber: string;
  rfc: string;
  curp: string;
  university: string;
  ineDocumentUrl: string;
  cedulaValidated: string;
}

type PendingAction = { doctorId: number; kind: 'approve' | 'reject' };

export const AdminPendingDoctors: React.FC = () => {
  const navigate = useNavigate();
  const [doctors, setDoctors] = useState<PendingDoctor[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<number, string>>({});
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null);
  const [feedback, setFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  const fetchPending = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await adminApi.get('/admin/doctors/pending');
      setDoctors(res.data || []);
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo cargar la lista de doctores pendientes.'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPending();
  }, [fetchPending]);

  const handleLogout = () => {
    localStorage.removeItem('admin_token');
    navigate('/admin/login');
  };

  const confirmAction = async () => {
    if (!pendingAction) return;
    const { doctorId, kind } = pendingAction;
    setPendingAction(null);
    try {
      if (kind === 'approve') {
        await adminApi.put(`/admin/doctors/${doctorId}/approve`);
        setFeedback({ type: 'success', message: 'Cédula aprobada. Se le avisó al doctor por correo.' });
      } else {
        await adminApi.put(`/admin/doctors/${doctorId}/reject`, { reason: reasons[doctorId] || '' });
        setFeedback({ type: 'success', message: 'Cédula rechazada. Se le avisó al doctor por correo.' });
      }
      fetchPending();
    } catch (err: unknown) {
      setFeedback({ type: 'error', message: getErrorMessage(err, 'No se pudo completar la acción.') });
    }
  };

  return (
    <div className="admin-pending-page">
      <header className="admin-pending-header">
        <h1>
          <span className="material-icons-outlined">fact_check</span>
          Cédulas pendientes de revisión
        </h1>
        <nav className="admin-nav">
          <Link to="/admin/doctores">Doctores</Link>
          <Link to="/admin/pendientes" className="active">Cédulas pendientes</Link>
        </nav>
        <button className="admin-logout-btn" onClick={handleLogout}>
          <span className="material-icons-outlined">logout</span>
          Cerrar sesión
        </button>
      </header>

      <main className="admin-pending-body">
        {loading && <p className="admin-pending-status">Cargando...</p>}
        {error && <p className="admin-pending-status admin-pending-error">{error}</p>}
        {!loading && !error && doctors.length === 0 && (
          <p className="admin-pending-status">No hay doctores esperando revisión.</p>
        )}

        {doctors.map((doc) => (
          <div key={doc.id} className="admin-doctor-card">
            <div className="admin-doctor-info">
              <h2>{doc.fullName || doc.username}</h2>
              <p>{doc.email}</p>
              <dl>
                <div><dt>Cédula</dt><dd>{doc.licenseNumber || '—'}</dd></div>
                <div><dt>RFC</dt><dd>{doc.rfc || '—'}</dd></div>
                <div><dt>CURP</dt><dd>{doc.curp || '—'}</dd></div>
                <div><dt>Universidad</dt><dd>{doc.university || '—'}</dd></div>
              </dl>
              {doc.ineDocumentUrl ? (
                <a href={doc.ineDocumentUrl} target="_blank" rel="noopener noreferrer" className="admin-ine-link">
                  <span className="material-icons-outlined">badge</span>
                  Ver identificación oficial (INE)
                </a>
              ) : (
                <p className="admin-ine-missing">Sin identificación adjunta</p>
              )}
            </div>

            <div className="admin-doctor-actions">
              <textarea
                placeholder="Motivo del rechazo (opcional, se le envía al doctor por correo)"
                value={reasons[doc.id] || ''}
                onChange={(e) => setReasons((prev) => ({ ...prev, [doc.id]: e.target.value }))}
              />
              <div className="admin-doctor-buttons">
                <button
                  className="admin-btn-reject"
                  onClick={() => setPendingAction({ doctorId: doc.id, kind: 'reject' })}
                >
                  Rechazar
                </button>
                <button
                  className="admin-btn-approve"
                  onClick={() => setPendingAction({ doctorId: doc.id, kind: 'approve' })}
                >
                  Aprobar
                </button>
              </div>
            </div>
          </div>
        ))}
      </main>

      <ConfirmDialog
        isOpen={!!pendingAction}
        title={pendingAction?.kind === 'approve' ? 'Aprobar cédula' : 'Rechazar cédula'}
        message={
          pendingAction?.kind === 'approve'
            ? 'El doctor tendrá acceso completo a la plataforma y se le avisará por correo.'
            : 'El doctor volverá a "pendiente" y podrá reenviar su documentación. Se le avisará por correo.'
        }
        confirmText={pendingAction?.kind === 'approve' ? 'Aprobar' : 'Rechazar'}
        variant={pendingAction?.kind === 'approve' ? 'default' : 'danger'}
        onConfirm={confirmAction}
        onCancel={() => setPendingAction(null)}
      />

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
