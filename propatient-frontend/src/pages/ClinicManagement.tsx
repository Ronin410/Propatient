import React, { useEffect, useState } from 'react';
import api from '../api/axios';
import { useAuth } from '../context/AuthContext';
import { Popup } from '../components/Popup';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { LocationPicker } from '../components/LocationPicker';
import { getErrorMessage } from '../utils/errorMessage';
import './ClinicManagement.scss';

interface ClinicDoctor {
  id: number;
  fullName: string;
  email: string;
  isOwner: boolean;
}

interface ClinicInfo {
  id: number;
  name: string;
  subscriptionStatus: 'incomplete' | 'active' | 'past_due' | 'canceled';
  isOwner: boolean;
  doctors: ClinicDoctor[];
  baseIncludedDoctors: number;
  extraDoctors: number;
  basePriceDisplay: number;
  extraPriceDisplay: number;
  address: string;
  latitude: number | null;
  longitude: number | null;
}

// "No configurado" (503) es distinto de "no perteneces a ninguna" (404):
// el primero significa que el plan de clínica ni siquiera está disponible
// en este servidor todavía (falta configurar Stripe), el segundo es el
// estado normal de cualquier doctor que nunca ha creado ni le han
// invitado a una clínica.
type ViewState = 'loading' | 'not-configured' | 'no-clinic' | 'has-clinic' | 'error';

export const ClinicManagement: React.FC = () => {
  const { isStaff } = useAuth();

  const [checkoutResult] = useState(() => new URLSearchParams(window.location.search).get('checkout'));
  const [viewState, setViewState] = useState<ViewState>('loading');
  const [errorMessage, setErrorMessage] = useState('');
  const [clinic, setClinic] = useState<ClinicInfo | null>(null);

  const [newClinicName, setNewClinicName] = useState('');
  const [creating, setCreating] = useState(false);

  const [inviteEmail, setInviteEmail] = useState('');
  const [inviting, setInviting] = useState(false);

  const [doctorToRemove, setDoctorToRemove] = useState<ClinicDoctor | null>(null);
  const [portalLoading, setPortalLoading] = useState(false);

  // Salir de la clínica por cuenta propia (solo para un doctor que NO es
  // dueño — el dueño cancela la clínica completa desde "Gestionar pagos",
  // ver handleManagePayments) — mismo efecto que doctorToRemove pero
  // iniciado por el propio doctor, sin depender de que el dueño lo quite.
  const [confirmLeaveOpen, setConfirmLeaveOpen] = useState(false);
  const [leaving, setLeaving] = useState(false);

  // Ubicación única de la clínica — solo el dueño la edita; el directorio
  // público la muestra para TODOS los doctores de la clínica en vez de la
  // dirección individual de cada quien (ver public_handler.go).
  const [locationAddress, setLocationAddress] = useState('');
  const [locationLat, setLocationLat] = useState<number | null>(null);
  const [locationLng, setLocationLng] = useState<number | null>(null);
  const [savingLocation, setSavingLocation] = useState(false);

  const [popupConfig, setPopupConfig] = useState({
    isOpen: false,
    type: 'success' as 'success' | 'error',
    title: '',
    message: '',
  });

  const fetchClinic = () => {
    api.get('/clinic')
      .then((res) => {
        setClinic(res.data);
        setViewState('has-clinic');
      })
      .catch((err) => {
        if (err?.response?.status === 404) {
          setViewState('no-clinic');
        } else if (err?.response?.status === 503) {
          setViewState('not-configured');
        } else {
          setErrorMessage(getErrorMessage(err, 'No se pudo consultar tu clínica.'));
          setViewState('error');
        }
      });
    // Limpia los parámetros de la URL para que el aviso de checkout no se
    // repita al recargar (mismo criterio que BillingPage).
    window.history.replaceState({}, '', window.location.pathname);
  };

  useEffect(() => {
    if (isStaff) return;
    fetchClinic();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isStaff]);

  useEffect(() => {
    if (!clinic) return;
    setLocationAddress(clinic.address || '');
    setLocationLat(clinic.latitude ?? null);
    setLocationLng(clinic.longitude ?? null);
  }, [clinic]);

  const handleSaveLocation = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingLocation(true);
    try {
      await api.put('/clinic/location', {
        address: locationAddress,
        latitude: locationLat != null ? String(locationLat) : '',
        longitude: locationLng != null ? String(locationLng) : '',
      });
      setPopupConfig({
        isOpen: true,
        type: 'success',
        title: 'Ubicación guardada',
        message: 'Todos los doctores de la clínica mostrarán esta dirección en el directorio público.',
      });
      fetchClinic();
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'No se pudo guardar',
        message: getErrorMessage(err, 'Ocurrió un error al guardar la ubicación.'),
      });
    } finally {
      setSavingLocation(false);
    }
  };

  const handleCreateClinic = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    try {
      const res = await api.post('/clinic', { name: newClinicName });
      window.location.href = res.data.url;
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'No se pudo crear la clínica',
        message: getErrorMessage(err, 'Ocurrió un error al iniciar el proceso de pago.'),
      });
      setCreating(false);
    }
  };

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    setInviting(true);
    try {
      await api.post('/clinic/invite', { email: inviteEmail });
      setInviteEmail('');
      setPopupConfig({
        isOpen: true,
        type: 'success',
        title: 'Invitación enviada',
        message: `Le enviamos un correo a ${inviteEmail} para que confirme que se une a la clínica.`,
      });
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'No se pudo invitar',
        message: getErrorMessage(err, 'Ocurrió un error al invitar al doctor.'),
      });
    } finally {
      setInviting(false);
    }
  };

  const handleRemoveConfirmed = async () => {
    if (!doctorToRemove) return;
    try {
      await api.delete(`/clinic/doctors/${doctorToRemove.id}`);
      setDoctorToRemove(null);
      fetchClinic();
    } catch (err: unknown) {
      setDoctorToRemove(null);
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error',
        message: getErrorMessage(err, 'No se pudo quitar al doctor de la clínica.'),
      });
    }
  };

  const handleLeaveClinic = async () => {
    setLeaving(true);
    try {
      await api.post('/clinic/leave');
      setConfirmLeaveOpen(false);
      // fetchClinic() volvería a traer 404 (ya no pertenece a ninguna
      // clínica) y eso mismo transiciona la vista a "no-clinic" — el flujo
      // natural para que pueda crear su propia clínica o esperar otra
      // invitación.
      fetchClinic();
    } catch (err: unknown) {
      setConfirmLeaveOpen(false);
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error',
        message: getErrorMessage(err, 'No se pudo salir de la clínica.'),
      });
    } finally {
      setLeaving(false);
    }
  };

  const handleManagePayments = async () => {
    setPortalLoading(true);
    try {
      const res = await api.post('/clinic/portal');
      window.location.href = res.data.url;
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error',
        message: getErrorMessage(err, 'No se pudo abrir el portal de facturación.'),
      });
      setPortalLoading(false);
    }
  };

  if (isStaff) {
    return (
      <div className="clinic-container">
        <div className="card clinic-card">
          <span className="material-icons-outlined clinic-icon">apartment</span>
          <h1>Mi Clínica</h1>
          <p>Solo el doctor dueño de la clínica puede administrarla.</p>
        </div>
      </div>
    );
  }

  if (viewState === 'loading') {
    return (
      <div className="clinic-container">
        <p className="clinic-muted">Cargando...</p>
      </div>
    );
  }

  if (viewState === 'error') {
    return (
      <div className="clinic-container">
        <div className="card clinic-card">
          <p className="clinic-error">{errorMessage}</p>
        </div>
      </div>
    );
  }

  if (viewState === 'not-configured') {
    return (
      <div className="clinic-container">
        <div className="card clinic-card">
          <span className="material-icons-outlined clinic-icon">apartment</span>
          <h1>Mi Clínica</h1>
          <p>El plan de clínica todavía no está disponible. Vuelve a intentarlo más adelante.</p>
        </div>
      </div>
    );
  }

  if (viewState === 'no-clinic') {
    return (
      <div className="clinic-container">
        <div className="card clinic-card">
          <span className="material-icons-outlined clinic-icon">apartment</span>
          <h1>Mi Clínica</h1>
          <p>
            Si tu consultorio tiene varios doctores, conviértelo en una clínica: un solo cobro cubre
            hasta 5 doctores, y cada doctor adicional se cobra por separado.
          </p>
          <form className="clinic-create-form" onSubmit={handleCreateClinic}>
            <div className="form-group">
              <label>Nombre de la clínica</label>
              <input
                type="text"
                value={newClinicName}
                onChange={(e) => setNewClinicName(e.target.value)}
                required
                placeholder="Ej. Consultorio Médico Familiar"
              />
            </div>
            <button type="submit" className="btn-primary" disabled={creating}>
              {creating ? 'Redirigiendo...' : 'Crear clínica'}
            </button>
          </form>
        </div>

        <Popup
          isOpen={popupConfig.isOpen}
          type={popupConfig.type}
          title={popupConfig.title}
          message={popupConfig.message}
          onClose={() => setPopupConfig((prev) => ({ ...prev, isOpen: false }))}
        />
      </div>
    );
  }

  // viewState === 'has-clinic'
  if (!clinic) return null;

  return (
    <div className="clinic-management-container">
      <header className="page-header">
        <div>
          <h1>{clinic.name}</h1>
          <p className="subtitle">
            Plan de clínica: ${clinic.basePriceDisplay.toLocaleString('es-MX')} MXN/mes cubre hasta{' '}
            {clinic.baseIncludedDoctors} doctores, más ${clinic.extraPriceDisplay.toLocaleString('es-MX')} MXN/mes
            por cada doctor adicional.
          </p>
        </div>
      </header>

      {checkoutResult === 'cancelled' && (
        <div className="clinic-alert">No se completó el pago. Puedes intentarlo de nuevo cuando quieras.</div>
      )}
      {checkoutResult === 'success' && (
        <div className="clinic-alert success">
          ¡Pago recibido! Puede tardar unos segundos en reflejarse — si el estado sigue apareciendo como
          pendiente, recarga la página.
        </div>
      )}

      <section className="card clinic-summary-card">
        <div className={`clinic-status ${clinic.subscriptionStatus}`}>
          <span className="material-icons-outlined">
            {clinic.subscriptionStatus === 'active' ? 'check_circle' : clinic.subscriptionStatus === 'incomplete' ? 'schedule' : 'error'}
          </span>
          <div>
            <strong>
              {clinic.subscriptionStatus === 'active' && 'Suscripción activa'}
              {clinic.subscriptionStatus === 'incomplete' && 'Pago pendiente de confirmar'}
              {clinic.subscriptionStatus === 'past_due' && 'Pago vencido'}
              {clinic.subscriptionStatus === 'canceled' && 'Suscripción cancelada'}
            </strong>
            <p>
              {clinic.doctors.length} doctor{clinic.doctors.length === 1 ? '' : 'es'} en la clínica
              {clinic.extraDoctors > 0 && ` (${clinic.extraDoctors} adicional${clinic.extraDoctors === 1 ? '' : 'es'} sobre el plan base)`}
            </p>
          </div>
        </div>
        {clinic.isOwner ? (
          <button className="btn-outline-sm" onClick={handleManagePayments} disabled={portalLoading}>
            {portalLoading ? 'Abriendo...' : 'Gestionar pagos'}
          </button>
        ) : (
          <button className="btn-outline-sm danger" onClick={() => setConfirmLeaveOpen(true)}>
            Salir de la clínica
          </button>
        )}
      </section>

      <section className="card">
        <h3>Ubicación de la clínica</h3>
        <p className="clinic-muted">
          Un consultorio de clínica tiene una sola dirección: todos sus doctores la muestran en el
          directorio público en vez de su dirección propia.
        </p>
        {clinic.isOwner ? (
          <form className="clinic-create-form" onSubmit={handleSaveLocation}>
            <div className="form-group">
              <label>Dirección física</label>
              <input
                type="text"
                value={locationAddress}
                onChange={(e) => setLocationAddress(e.target.value)}
                placeholder="Calle, número, colonia, ciudad"
              />
            </div>
            <LocationPicker
              address={locationAddress}
              latitude={locationLat}
              longitude={locationLng}
              onChange={(lat, lng) => {
                setLocationLat(lat);
                setLocationLng(lng);
              }}
            />
            <button type="submit" className="btn-primary" disabled={savingLocation}>
              {savingLocation ? 'Guardando...' : 'Guardar ubicación'}
            </button>
          </form>
        ) : (
          <p>{clinic.address || 'Aún no se ha configurado la dirección de la clínica.'}</p>
        )}
      </section>

      {clinic.isOwner && (
        <section className="card invite-card">
          <h3>Invitar a un doctor</h3>
          <p className="clinic-muted">
            El doctor debe tener ya una cuenta en ProPatient con ese correo — le llegará un aviso para
            confirmar que se une.
          </p>
          <form className="invite-form" onSubmit={handleInvite}>
            <div className="form-group">
              <label>Correo del doctor</label>
              <input type="email" value={inviteEmail} onChange={(e) => setInviteEmail(e.target.value)} required />
            </div>
            <button type="submit" className="btn-primary" disabled={inviting}>
              {inviting ? 'Enviando...' : 'Enviar invitación'}
            </button>
          </form>
        </section>
      )}

      <section className="card clinic-doctors-card">
        <h3>Doctores de la clínica</h3>
        <table className="clinic-doctors-table">
          <thead>
            <tr>
              <th>Nombre</th>
              <th>Correo</th>
              <th>Rol</th>
              {clinic.isOwner && <th>Acciones</th>}
            </tr>
          </thead>
          <tbody>
            {clinic.doctors.map((d) => (
              <tr key={d.id}>
                <td>{d.fullName}</td>
                <td>{d.email}</td>
                <td>{d.isOwner ? <span className="status-badge status-owner">Dueño</span> : 'Doctor'}</td>
                {clinic.isOwner && (
                  <td className="actions-cell">
                    {!d.isOwner && (
                      <button className="btn-outline-sm danger" onClick={() => setDoctorToRemove(d)}>
                        Quitar
                      </button>
                    )}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <ConfirmDialog
        isOpen={!!doctorToRemove}
        variant="danger"
        title="Quitar doctor de la clínica"
        message={`¿Seguro que quieres quitar a ${doctorToRemove?.fullName || ''} de la clínica? Perderá el acceso a ProPatient hasta que se suscriba por su cuenta o lo inviten de nuevo.`}
        confirmText="Quitar"
        onConfirm={handleRemoveConfirmed}
        onCancel={() => setDoctorToRemove(null)}
      />

      <ConfirmDialog
        isOpen={confirmLeaveOpen}
        variant="danger"
        title="Salir de la clínica"
        message="¿Seguro que quieres salir de la clínica? Perderás el acceso a ProPatient hasta que te suscribas por tu cuenta o te inviten de nuevo."
        confirmText={leaving ? 'Saliendo...' : 'Sí, salir'}
        onConfirm={handleLeaveClinic}
        onCancel={() => setConfirmLeaveOpen(false)}
      />

      <Popup
        isOpen={popupConfig.isOpen}
        type={popupConfig.type}
        title={popupConfig.title}
        message={popupConfig.message}
        onClose={() => setPopupConfig((prev) => ({ ...prev, isOpen: false }))}
      />
    </div>
  );
};
