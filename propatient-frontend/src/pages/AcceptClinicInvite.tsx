import React, { useEffect, useState } from 'react';
import axios from 'axios';
import { useParams, useNavigate, Link } from 'react-router-dom';
import api from '../api/axios';
import { useAuth } from '../context/AuthContext';
import { getErrorMessage } from '../utils/errorMessage';
import './ClinicManagement.scss';

interface ClinicInviteInfo {
  clinicName: string;
  ownerName: string;
}

// A diferencia de la invitación de personal (AcceptStaffInvite), aquí el
// doctor invitado YA TIENE cuenta propia en ProPatient — esta pantalla
// requiere sesión iniciada (ver App.tsx, va dentro de ProtectedRoute) y
// solo confirma que de verdad quiere unirse, con su propio login normal.
export const AcceptClinicInvite: React.FC = () => {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const { isStaff, logout } = useAuth();

  const [invite, setInvite] = useState<ClinicInviteInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  // true si el error es porque la sesión abierta es de OTRA cuenta (no la
  // invitada) — ver clinicInviteWrongAccountMessage en el backend. En ese
  // caso ofrecemos cerrar sesión en vez de un genérico "no válida".
  const [wrongAccount, setWrongAccount] = useState(false);
  const [accepting, setAccepting] = useState(false);
  const [acceptError, setAcceptError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    if (!token || isStaff) return;
    api.get(`/clinic/invitations/${token}`)
      .then((res) => setInvite(res.data))
      .catch((err) => {
        setLoadError(getErrorMessage(err, 'Esta invitación no es válida.'));
        setWrongAccount(axios.isAxiosError(err) && !!err.response?.data?.wrongAccount);
      })
      .finally(() => setLoading(false));
  }, [token, isStaff]);

  const handleLogoutAndRetry = () => {
    logout();
    navigate('/login');
  };

  const handleAccept = async () => {
    setAccepting(true);
    setAcceptError(null);
    try {
      await api.post(`/clinic/invitations/${token}/accept`);
      setSuccess(true);
      setTimeout(() => navigate('/clinica'), 1800);
    } catch (err: unknown) {
      setAcceptError(getErrorMessage(err, 'No se pudo aceptar la invitación. Intenta de nuevo.'));
    } finally {
      setAccepting(false);
    }
  };

  if (isStaff) {
    return (
      <div className="clinic-container">
        <div className="card clinic-card">
          <span className="material-icons-outlined clinic-icon">apartment</span>
          <h1>Invitación a clínica</h1>
          <p>Esta invitación es para una cuenta de doctor, no de personal. Inicia sesión con la cuenta invitada.</p>
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="clinic-container">
        <div className="card clinic-card">
          <p className="clinic-muted">Verificando invitación...</p>
        </div>
      </div>
    );
  }

  if (loadError || !invite) {
    return (
      <div className="clinic-container">
        <div className="card clinic-card">
          <span className="material-icons-outlined clinic-icon">apartment</span>
          <h1>Invitación no disponible</h1>
          <p className="clinic-error">{loadError}</p>
          {wrongAccount ? (
            <div className="clinic-create-form">
              <button type="button" className="btn-primary" onClick={handleLogoutAndRetry}>
                Cerrar sesión e iniciar con esa cuenta
              </button>
            </div>
          ) : (
            <p><Link to="/inicio">Ir a mi panel</Link></p>
          )}
        </div>
      </div>
    );
  }

  if (success) {
    return (
      <div className="clinic-container">
        <div className="card clinic-card">
          <span className="material-icons-outlined clinic-icon">check_circle</span>
          <h1>¡Listo!</h1>
          <p>Te uniste a {invite.clinicName}. Te llevamos a tu clínica...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="clinic-container">
      <div className="card clinic-card">
        <span className="material-icons-outlined clinic-icon">apartment</span>
        <h1>Te invitaron a una clínica</h1>
        <p>
          <strong>{invite.ownerName}</strong> te invitó a unirte a <strong>{invite.clinicName}</strong> en
          ProPatient Clinic. Al aceptar, tu suscripción individual (si tenías una) se cancela y quedas cubierto por
          el plan de la clínica.
        </p>
        {acceptError && <p className="clinic-error">{acceptError}</p>}
        <div className="clinic-create-form">
          <button type="button" className="btn-primary" onClick={handleAccept} disabled={accepting}>
            {accepting ? 'Uniéndote...' : 'Aceptar y unirme'}
          </button>
        </div>
      </div>
    </div>
  );
};
