import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import api from '../api/axios';
import { useAuth } from '../context/AuthContext';
import { getErrorMessage } from '../utils/errorMessage';
import { formatToLocalDate } from '../utils/dateFormatter';
import { Popup } from '../components/Popup';
import './BillingPage.scss';

interface BillingStatus {
  // true si el doctor pertenece a una clínica: su acceso depende de la
  // suscripción de la clínica, no de una propia — ver GetBillingStatus,
  // que en ese caso no manda nada más (los demás campos quedan opcionales
  // porque simplemente no aplican).
  managedByClinic: boolean;
  subscriptionStatus?: 'trialing' | 'active' | 'past_due' | 'canceled';
  trialEndsAt?: string | null;
  hasPaymentMethod?: boolean;
  // Fecha real de la próxima renovación, consultada a Stripe en vivo (ver
  // GetBillingStatus en el backend) — null si no hay suscripción activa,
  // o si Stripe no respondió a tiempo (mejor esfuerzo, el resto del
  // estatus se sigue mostrando igual).
  currentPeriodEnd?: string | null;
  // Hasta cuándo sigue el acceso pese al cobro fallido (ver
  // billing.PastDuePaymentGraceDuration en el backend) — solo viene cuando
  // subscriptionStatus es "past_due" y se registró desde cuándo empezó.
  pastDueGraceEndsAt?: string | null;
  // Precio de lanzamiento (ver billing.Config.CheckoutPriceID en el
  // backend): mientras launchPromoActive sea true, suscribirse AHORA deja
  // el precio en launchPriceMXN para siempre en vez de regularPriceMXN.
  launchPromoActive?: boolean;
  launchPriceMXN?: number;
  regularPriceMXN?: number;
  launchPromoEndsAt?: string | null;
}

interface ReferralInfo {
  code: string;
  totalReferrals: number;
  rewardedReferrals: number;
  remainingSlots: number;
  maxReferrals: number;
}

function daysLeft(dateStr: string | null | undefined): number {
  if (!dateStr) return 0;
  const ms = new Date(dateStr).getTime() - Date.now();
  return Math.max(0, Math.ceil(ms / (1000 * 60 * 60 * 24)));
}

function discountPercent(regularMXN: number, launchMXN: number): number {
  return Math.round((1 - launchMXN / regularMXN) * 100);
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

  const [referral, setReferral] = useState<ReferralInfo | null>(null);
  const [copyPopupOpen, setCopyPopupOpen] = useState(false);

  // Código de invitación de OTRO doctor, capturado aquí — justo antes de
  // pagar, ya con la cuenta y cédula validadas (esta pantalla solo es
  // alcanzable después de eso, ver OnboardingGuard) — en vez de en el
  // registro inicial. Solo queda "pending"; la recompensa real se otorga
  // hasta que el pago se complete de verdad (ver StripeWebhook).
  const [applyCodeInput, setApplyCodeInput] = useState('');
  const [applyingCode, setApplyingCode] = useState(false);
  const [applyCodeMessage, setApplyCodeMessage] = useState<{ type: 'success' | 'info'; text: string } | null>(null);

  useEffect(() => {
    if (isStaff) return;
    api.get('/billing/status')
      .then((res) => setStatus(res.data))
      .catch((err) => setError(getErrorMessage(err, 'No se pudo consultar el estado de tu suscripción.')))
      .finally(() => setLoading(false));
    // Limpia los parámetros de la URL para que no se repita el aviso al recargar.
    window.history.replaceState({}, '', window.location.pathname);
  }, [isStaff]);

  // El código de invitación solo existe una vez que la suscripción está
  // activa (ver handlers.GetReferralCode en el backend) — se consulta
  // aparte, después de que /billing/status confirme ese estado.
  useEffect(() => {
    if (status?.subscriptionStatus !== 'active') return;
    api.get('/doctor/referral-code')
      .then((res) => setReferral(res.data))
      .catch(() => setReferral(null));
  }, [status?.subscriptionStatus]);

  const handleCopyReferralCode = async () => {
    if (!referral) return;
    try {
      await navigator.clipboard.writeText(referral.code);
      setCopyPopupOpen(true);
    } catch {
      // Sin permiso de portapapeles: el input de solo lectura de abajo
      // ya deja al doctor seleccionar y copiar el código a mano.
    }
  };

  const handleApplyReferralCode = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!applyCodeInput.trim()) return;
    setApplyingCode(true);
    setApplyCodeMessage(null);
    try {
      const res = await api.post('/billing/apply-referral-code', { code: applyCodeInput });
      if (res.data.applied) {
        setApplyCodeMessage({ type: 'success', text: '¡Código aplicado! Ganarás una semana gratis en cuanto se confirme tu pago.' });
      } else {
        setApplyCodeMessage({ type: 'info', text: 'Ese código no es válido o ya no está disponible.' });
      }
    } catch (err: unknown) {
      setApplyCodeMessage({ type: 'info', text: getErrorMessage(err, 'No se pudo aplicar el código, intenta de nuevo.') });
    } finally {
      setApplyingCode(false);
    }
  };

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
            Pídele al doctor que renueve su suscripción para poder seguir usando ProPatient Clinic.
          </p>
        </div>
      </div>
    );
  }

  if (status?.managedByClinic) {
    return (
      <div className="billing-container">
        <div className="card billing-card">
          <span className="material-icons-outlined billing-icon">apartment</span>
          <h1>Suscripción</h1>
          <p>
            Tu consultorio forma parte de una clínica — el pago y la administración de la
            suscripción los gestiona el dueño de la clínica desde "Mi Clínica", no aquí.
          </p>
          <div className="billing-actions">
            <Link to="/clinica" className="btn-primary" style={{ display: 'inline-block', textDecoration: 'none' }}>
              Ir a Mi Clínica
            </Link>
          </div>
        </div>
      </div>
    );
  }

  // Decide si el botón manda al Portal de Cliente de Stripe ("Gestionar
  // suscripción") o abre un Checkout nuevo ("Suscribirse"). OJO:
  // status.hasPaymentMethod es en realidad "alguna vez tuvo un Customer de
  // Stripe" (ver GetBillingStatus) — se queda en true PARA SIEMPRE una vez
  // que existe, incluso después de que Stripe cancele la suscripción por
  // completo. Por eso NO basta con mirar hasPaymentMethod solo: para
  // "canceled" la suscripción ya no existe en Stripe, así que el Portal no
  // tiene nada que gestionar (solo muestra método de pago e historial,
  // sin forma de volver a suscribirse) — hace falta un Checkout nuevo. En
  // "past_due" sí sigue viva, ahí el Portal puede reactivarla de verdad.
  const canManagePortal = status?.subscriptionStatus === 'active' ||
    (status?.subscriptionStatus === 'past_due' && status?.hasPaymentMethod === true);

  return (
    <div className="billing-container">
      <div className="card billing-card">
        {wasLocked && (
          <div className="billing-alert">
            Tu periodo de prueba terminó. Suscríbete para seguir usando ProPatient Clinic.
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
                    <p>Tu consultorio tiene acceso completo a ProPatient Clinic.</p>
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
                  {status.subscriptionStatus === 'past_due' && status.pastDueGraceEndsAt ? (
                    <p>
                      Tu banco rechazó el último cobro. Tienes {daysLeft(status.pastDueGraceEndsAt)} día
                      {daysLeft(status.pastDueGraceEndsAt) === 1 ? '' : 's'} para actualizar tu método de pago antes
                      de perder el acceso.
                    </p>
                  ) : status.subscriptionStatus === 'past_due' ? (
                    <p>Actualiza tu método de pago para reactivar el acceso completo.</p>
                  ) : (
                    // Una suscripción "canceled" ya no existe en Stripe (a
                    // diferencia de "past_due", donde el cobro solo falló
                    // pero la suscripción sigue viva) — no hay nada que
                    // "reactivar" desde el portal, hace falta un Checkout
                    // nuevo (ver canManagePortal más abajo).
                    <p>Vuelve a suscribirte para recuperar el acceso completo.</p>
                  )}
                </div>
              </div>
            )}

            {status.launchPromoActive &&
              status.launchPriceMXN != null &&
              status.regularPriceMXN != null &&
              !canManagePortal && (
                <div className="billing-promo">
                  <span className="billing-promo-badge">
                    {discountPercent(status.regularPriceMXN, status.launchPriceMXN)}% de descuento — precio de
                    lanzamiento
                  </span>
                  <div className="billing-promo-prices">
                    <span className="billing-promo-regular">
                      ${status.regularPriceMXN.toLocaleString('es-MX')} MXN/mes
                    </span>
                    <span className="billing-promo-launch">
                      ${status.launchPriceMXN.toLocaleString('es-MX')} MXN/mes
                    </span>
                  </div>
                  <p>
                    {status.launchPromoEndsAt && <>Válido para quien se suscriba antes del {formatToLocalDate(status.launchPromoEndsAt)}. </>}
                    Si te suscribes ahora, tu precio se queda en ${status.launchPriceMXN.toLocaleString('es-MX')}{' '}
                    MXN/mes <strong>para siempre</strong> — nunca sube, aunque el precio normal de la plataforma sea
                    de ${status.regularPriceMXN.toLocaleString('es-MX')} MXN/mes.
                  </p>
                </div>
              )}

            <div className="billing-actions">
              {canManagePortal ? (
                <button className="btn-primary" onClick={handleManage} disabled={actionLoading}>
                  {actionLoading ? 'Abriendo...' : 'Gestionar suscripción'}
                </button>
              ) : (
                <button className="btn-primary" onClick={handleSubscribe} disabled={actionLoading}>
                  {actionLoading ? 'Redirigiendo...' : 'Suscribirse'}
                </button>
              )}
            </div>

            {status.subscriptionStatus === 'active' && (
              <div className="billing-referral">
                <h2>¿Necesitas tu factura (CFDI) de esta suscripción?</h2>
                <p>
                  Poco después de cada pago, Facturapi te escribe por correo pidiéndote tus datos
                  fiscales (RFC, régimen fiscal, código postal) para generar tu CFDI automáticamente.
                  Complétalos ahí en cuanto te lleguen — si esperas demasiado, ya no se puede generar
                  la factura de ese cobro en automático y hay que tramitarla a mano. Si no te llegó el
                  correo o tienes dudas, escríbenos.
                </p>
              </div>
            )}

            {status.subscriptionStatus !== 'active' && (
              <div className="billing-referral">
                <h2>¿Tienes un código de invitación?</h2>
                <p>
                  Captúralo antes de pagar — cuando tu suscripción se active, tú y quien te invitó ganan
                  1 semana extra gratis cada uno.
                </p>
                <form className="billing-referral-apply-form" onSubmit={handleApplyReferralCode}>
                  <input
                    type="text"
                    value={applyCodeInput}
                    onChange={(e) => setApplyCodeInput(e.target.value)}
                    placeholder="Código de un colega"
                  />
                  <button type="submit" className="btn-secondary" disabled={applyingCode || !applyCodeInput.trim()}>
                    {applyingCode ? 'Aplicando...' : 'Aplicar código'}
                  </button>
                </form>
                {applyCodeMessage && (
                  <p className={`billing-referral-apply-message ${applyCodeMessage.type}`}>{applyCodeMessage.text}</p>
                )}
              </div>
            )}

            {status.subscriptionStatus === 'active' && referral && (
              <div className="billing-referral">
                <h2>Invita a un colega, gana una semana gratis</h2>
                <p>
                  Comparte tu código. Cuando un colega se registre con él y pague su suscripción,
                  ambos ganan 1 semana extra gratis.
                </p>
                <div className="billing-referral-code-row">
                  <input
                    type="text"
                    readOnly
                    value={referral.code}
                    onClick={(e) => (e.target as HTMLInputElement).select()}
                  />
                  <button className="btn-secondary" onClick={handleCopyReferralCode}>
                    Copiar
                  </button>
                </div>
                <p className="billing-referral-progress">
                  Has invitado a {referral.totalReferrals} colega{referral.totalReferrals === 1 ? '' : 's'} ·{' '}
                  {referral.rewardedReferrals} semana{referral.rewardedReferrals === 1 ? '' : 's'} ganada
                  {referral.rewardedReferrals === 1 ? '' : 's'} · quedan {referral.remainingSlots} de{' '}
                  {referral.maxReferrals}
                </p>
              </div>
            )}
          </>
        ) : null}
      </div>

      <Popup
        isOpen={copyPopupOpen}
        type="success"
        title="¡Código copiado!"
        message="Ya puedes compartirlo con tu colega."
        onClose={() => setCopyPopupOpen(false)}
      />
    </div>
  );
};
