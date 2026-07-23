import React, { useEffect, useState } from 'react';
import api from '../api/axios';
import { useAuth } from '../context/AuthContext';
import { getErrorMessage } from '../utils/errorMessage';
import { formatToLocalDate } from '../utils/dateFormatter';
import './BillingPage.scss';

interface BillingStatus {
  subscriptionStatus: 'trialing' | 'active' | 'past_due' | 'canceled';
  trialEndsAt: string | null;
  hasPaymentMethod: boolean;
  // Fecha real de la próxima renovación, consultada a Stripe en vivo (ver
  // GetBillingStatus en el backend) — null si no hay suscripción activa,
  // o si Stripe no respondió a tiempo (mejor esfuerzo, el resto del
  // estatus se sigue mostrando igual).
  currentPeriodEnd: string | null;
}

function daysLeft(dateStr: string | null): number {
  if (!dateStr) return 0;
  const ms = new Date(dateStr).getTime() - Date.now();
  return Math.max(0, Math.ceil(ms / (1000 * 60 * 60 * 24)));
}

export const BillingPage: React.FC = () => {
  const { isStaff } = useAuth();
  // Se captura UNA sola vez al montar (useState con inicializador, no un
  // const derivado en cada render): el useEffect de abajo limpia la URL con
  // replaceState, así que leer window.location.search directamente en cada
  // render haría que el aviso desapareciera apenas se re-renderiza el
  // componente (ej. cuando termina de cargar el estado de la suscripción).
  const [wasLocked] = useState(() => new URLSearchParams(window.location.search).get('locked') === '1');
  const [checkoutResult] = useState(() => new URLSearchParams(window.location.search).get('checkout'));

  const [status, setStatus] = useState<BillingStatus | null>(null);
  const [loading, setLoading] = useState(!isStaff);
  const [actionLoading, setActionLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isStaff) return;
    api.get('/billing/status')
      .then((res) => setStatus(res.data))
      .catch((err) => setError(getErrorMessage(err, 'No se pudo consultar el estado de tu suscripción.')))
      .finally(() => setLoading(false));
    // Limpia los parámetros de la URL para que no se repita el aviso al recargar.
    window.history.replaceState({}, '', window.location.pathname);
  }, [isStaff]);

  const handleSubscribe = async () => {
    setActionLoading(true);
    setError(null);
    try {
      const res = await api.post('/billing/checkout');
      window.location.href = res.data.url;
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo iniciar el proceso de pago. Intenta de nuevo.'));
      setActionLoading(false);
    }
  };

  const handleManage = async () => {
    setActionLoading(true);
    setError(null);
    try {
      const res = await api.post('/billing/portal');
      window.location.href = res.data.url;
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo abrir el portal de facturación.'));
      setActionLoading(false);
    }
  };

  if (isStaff) {
    return (
      <div className="billing-container">
        <div className="card billing-card">
          <span className="material-icons-outlined billing-icon locked">lock_clock</span>
          <h1>Acceso suspendido</h1>
          <p>
            El periodo de prueba o la suscripción del consultorio ya no está activa.
            Pídele al doctor que renueve su suscripción para poder seguir usando ProPatient.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="billing-container">
      <div className="card billing-card">
        {wasLocked && (
          <div className="billing-alert">
            Tu periodo de prueba terminó. Suscríbete para seguir usando ProPatient.
          </div>
        )}
        {checkoutResult === 'cancelled' && (
          <div className="billing-alert">No se completó el pago. Puedes intentarlo de nuevo cuando quieras.</div>
        )}
        {checkoutResult === 'success' && (
          <div className="billing-alert success">
            ¡Pago recibido! Puede tardar unos segundos en reflejarse — si sigues viendo "Prueba", recarga la página.
          </div>
        )}

        <h1>Suscripción</h1>

        {loading ? (
          <p className="billing-muted">Cargando...</p>
        ) : error ? (
          <p className="billing-error">{error}</p>
        ) : status ? (
          <>
            {status.subscriptionStatus === 'active' && (
              <div className="billing-status active">
                <span className="material-icons-outlined">check_circle</span>
                <div>
                  <strong>Suscripción activa</strong>
                  {status.currentPeriodEnd ? (
                    <p>
                      Se renueva automáticamente en {daysLeft(status.currentPeriodEnd)} días
                      ({formatToLocalDate(status.currentPeriodEnd)}).
                    </p>
                  ) : (
                    <p>Tu consultorio tiene acceso completo a ProPatient.</p>
                  )}
                </div>
              </div>
            )}

            {status.subscriptionStatus === 'trialing' && (
              <div className="billing-status trial">
                <span className="material-icons-outlined">schedule</span>
                <div>
                  <strong>{daysLeft(status.trialEndsAt)} días de prueba gratis restantes</strong>
                  <p>Suscríbete para no perder el acceso cuando termine.</p>
                </div>
              </div>
            )}

            {(status.subscriptionStatus === 'past_due' || status.subscriptionStatus === 'canceled') && (
              <div className="billing-status danger">
                <span className="material-icons-outlined">error</span>
                <div>
                  <strong>{status.subscriptionStatus === 'past_due' ? 'Pago pendiente' : 'Suscripción cancelada'}</strong>
                  <p>Actualiza tu método de pago para reactivar el acceso completo.</p>
                </div>
              </div>
            )}

            <div className="billing-actions">
              {status.subscriptionStatus === 'active' || status.hasPaymentMethod ? (
                <button className="btn-primary" onClick={handleManage} disabled={actionLoading}>
                  {actionLoading ? 'Abriendo...' : 'Gestionar suscripción'}
                </button>
              ) : (
                <button className="btn-primary" onClick={handleSubscribe} disabled={actionLoading}>
                  {actionLoading ? 'Redirigiendo...' : 'Suscribirse'}
                </button>
              )}
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
};
