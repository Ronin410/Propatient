import React, { useState, useEffect, useRef } from 'react';
import api from '../api/axios';
import './DoctorProfile.scss';
import { Popup } from '../components/Popup';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { getErrorMessage } from '../utils/errorMessage';
import { toAbsoluteFileUrl } from '../utils/fileUrl';
import { sanitizePhoneInput } from '../utils/phoneInput';
import { LocationPicker } from '../components/LocationPicker';
import { DoctorGallery } from '../components/DoctorGallery';
import { useAuth } from '../context/AuthContext';

interface ProfileData {
  rfc: string;
  curp: string;
  licenseNumber: string;
  fullName: string;
  email: string;
  phone: string;
  medicalSpecialty: string;
  university: string;
  address: string;
  recipeLegend: string;
  resume: string;
  avatarUrl?: string;
  logoUrl?: string;
  googleCalendarConnected?: boolean;
  publicListed?: boolean;
  publicBio?: string;
  publicSlug?: string;
  latitude?: number | null;
  longitude?: number | null;
  facebookUrl?: string;
  instagramUrl?: string;
  linkedinUrl?: string;
  twitterUrl?: string;
  tiktokUrl?: string;
  youtubeUrl?: string;
  websiteUrl?: string;
}

export const DoctorProfile = () => {
  const [profile, setProfile] = useState<ProfileData>({
    rfc: 'BEMA930812XXX',
    curp: 'BEMA930812HDFXNX01',
    licenseNumber: '1282',
    fullName: localStorage.getItem('suggested_fullname') || '',
    email: '',
    phone: '',
    medicalSpecialty: '',
    university: '',
    address: '',
    recipeLegend: 'Favor de no automedicarse. En caso de presentar reacciones adversas suspenda el medicamento y consulte a su médico.',
    resume: '',
    publicListed: false,
    publicBio: '',
    latitude: null,
    longitude: null,
    facebookUrl: '',
    instagramUrl: '',
    linkedinUrl: '',
    twitterUrl: '',
    tiktokUrl: '',
    youtubeUrl: '',
    websiteUrl: ''
  });
  // Estado único para controlar la configuración del popup genérico
  const [popupConfig, setPopupConfig] = useState({
    isOpen: false,
    type: 'success' as 'success' | 'error',
    title: '',
    message: ''
  });

  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error', text: string } | null>(null);

  const [avatarPreview, setAvatarPreview] = useState<string | null>(null);
  const [logoPreview, setLogoPreview] = useState<string | null>(null);
  
  const [avatarFile, setAvatarFile] = useState<File | null>(null);
  const [logoFile, setLogoFile] = useState<File | null>(null);

  const [showSuccessPopup, setShowSuccessPopup] = useState(false);

  const avatarInputRef = useRef<HTMLInputElement>(null);
  const logoInputRef = useRef<HTMLInputElement>(null);

  const [connectingCalendar, setConnectingCalendar] = useState(false);

  const [exportingData, setExportingData] = useState(false);
  const [deletingAccount, setDeletingAccount] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const { logout, setDoctorName } = useAuth();

  const calculateCompletion = () => {
    const fieldsToValidate = [
      profile.fullName, profile.email, profile.phone, 
      profile.medicalSpecialty, profile.university, profile.address,
      profile.resume
    ];
    const completedFields = fieldsToValidate.filter(field => field && field.trim() !== "").length;
    const imagesCompleted = (avatarPreview ? 1 : 0) + (logoPreview ? 1 : 0);
    
    return Math.round(((completedFields + imagesCompleted) / 9) * 100);
  };

  useEffect(() => {
    const fetchProfileData = async () => {
      setIsLoading(true);
      try {
        const res = await api.get('/doctor/me');
        if (res.data) {
          setProfile(prev => ({ ...prev, ...res.data }));
          if (res.data.avatarUrl) {
            // Si la url ya viene completa de casualidad, la dejas, si no, le pegas el Host
            const fullAvatar = toAbsoluteFileUrl(res.data.avatarUrl);
            setAvatarPreview(fullAvatar);
          }
          if (res.data.logoUrl) {
            const fullLogo = toAbsoluteFileUrl(res.data.logoUrl);
            setLogoPreview(fullLogo);
          }

          profile.fullName = res.data.fullName;
          profile.acercaDeMi = res.data.resume;
          profile.rfc = res.data.rfc;
          profile.curp = res.data.curp;
          profile.licenseNumber = res.data.licenseNumber;
          profile.phone = res.data.phone;
          profile.medicalSpecialty = res.data.medicalSpecialty;
          profile.university = res.data.university;
          profile.address = res.data.address;
          profile.recipeLegend = res.data.recipeLegend;
          profile.avatarUrl = res.data.avatarUrl;
          profile.logoUrl = res.data.logoUrl;
        }
        
      } catch (err) {
        console.error("Error al obtener el perfil", err);
      } finally {
        setIsLoading(false);
      }
    };

    fetchProfileData();
  }, []);

  // El callback de Google Calendar (backend) redirige aquí con
  // ?calendar=connected o ?calendar=error tras el consentimiento.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const calendarResult = params.get('calendar');
    if (!calendarResult) return;

    if (calendarResult === 'connected') {
      setPopupConfig({
        isOpen: true,
        type: 'success',
        title: 'Google Calendar Conectado',
        message: 'A partir de ahora, tus citas nuevas se agregarán automáticamente a tu Google Calendar.'
      });
      setProfile(prev => ({ ...prev, googleCalendarConnected: true }));
    } else {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'No se pudo conectar Google Calendar',
        message: 'Intenta de nuevo desde el botón "Conectar con Google Calendar".'
      });
    }

    // Limpia el parámetro de la URL para que no se repita el aviso al recargar.
    window.history.replaceState({}, '', window.location.pathname);
  }, []);

  const handleConnectCalendar = async () => {
    setConnectingCalendar(true);
    try {
      const res = await api.get('/doctor/google-calendar/connect');
      window.location.href = res.data.url;
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Google Calendar no disponible',
        message: getErrorMessage(err, 'La integración con Google Calendar no está disponible en este momento.')
      });
      setConnectingCalendar(false);
    }
  };

  const handleDisconnectCalendar = async () => {
    setConnectingCalendar(true);
    try {
      await api.post('/doctor/google-calendar/disconnect');
      setProfile(prev => ({ ...prev, googleCalendarConnected: false }));
      setPopupConfig({
        isOpen: true,
        type: 'success',
        title: 'Google Calendar Desconectado',
        message: 'Tus citas ya no se sincronizarán con Google Calendar.'
      });
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error',
        message: getErrorMessage(err, 'No se pudo desconectar Google Calendar.')
      });
    } finally {
      setConnectingCalendar(false);
    }
  };

  // Descarga un volcado en JSON de todo lo asociado a la cuenta (perfil,
  // pacientes, citas, personal) — derecho de Acceso de las garantías ARCO.
  const handleExportData = async () => {
    setExportingData(true);
    try {
      const res = await api.get('/user/export-data');
      const blob = new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'propatient-mis-datos.json';
      link.click();
      URL.revokeObjectURL(url);
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error',
        message: getErrorMessage(err, 'No se pudieron exportar tus datos.')
      });
    } finally {
      setExportingData(false);
    }
  };

  const handleDeleteAccount = async () => {
    setShowDeleteConfirm(false);
    setDeletingAccount(true);
    try {
      await api.delete('/user/account');
      logout();
      // Recarga completa (no navigate() de react-router): /profile está
      // envuelta por OnboardingGuard, que redirige a /login en cuanto
      // isAuthenticated se vuelve false — esa redirección le gana la
      // carrera a un navigate('/') hecho en el mismo tick. Una recarga
      // completa evita el problema por completo (mismo patrón que ya usa
      // el interceptor de 401 en api/axios.ts).
      window.location.href = '/';
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'No se pudo eliminar tu cuenta',
        message: getErrorMessage(err, 'Intenta de nuevo.')
      });
      setDeletingAccount(false);
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { name, value } = e.target;
    setProfile(prev => ({ ...prev, [name]: name === 'phone' ? sanitizePhoneInput(value) : value }));
  };

  const handlePublicListedChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setProfile(prev => ({ ...prev, publicListed: e.target.checked }));
  };

  const handleLocationChange = (lat: number, lng: number) => {
    setProfile(prev => ({ ...prev, latitude: lat, longitude: lng }));
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>, type: 'avatar' | 'logo') => {
    if (e.target.files && e.target.files[0]) {
      const file = e.target.files[0];
      const previewUrl = URL.createObjectURL(file);

      if (type === 'avatar') {
        setAvatarFile(file);         // <--- GUARDAR EL ARCHIVO REAL
        setAvatarPreview(previewUrl);
      } else {
        setLogoFile(file);           // <--- GUARDAR EL ARCHIVO REAL
        setLogoPreview(previewUrl);
      }
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSaving(true);
    setMessage(null);

    try {
      const formData = new FormData();
      
      // 2. Adjuntar todos tus campos de texto actuales
      formData.append('rfc', profile.rfc);
      formData.append('curp', profile.curp);
      formData.append('licenseNumber', profile.licenseNumber);
      formData.append('fullName', profile.fullName);
      formData.append('email', profile.email);
      formData.append('phone', profile.phone);
      formData.append('medicalSpecialty', profile.medicalSpecialty);
      formData.append('university', profile.university);
      formData.append('address', profile.address);
      formData.append('recipeLegend', profile.recipeLegend);
      formData.append('resume', profile.resume);
      formData.append('publicListed', profile.publicListed ? 'true' : 'false');
      formData.append('publicBio', profile.publicBio || '');
      formData.append('facebookUrl', profile.facebookUrl || '');
      formData.append('instagramUrl', profile.instagramUrl || '');
      formData.append('linkedinUrl', profile.linkedinUrl || '');
      formData.append('twitterUrl', profile.twitterUrl || '');
      formData.append('tiktokUrl', profile.tiktokUrl || '');
      formData.append('youtubeUrl', profile.youtubeUrl || '');
      formData.append('websiteUrl', profile.websiteUrl || '');
      if (profile.latitude != null && profile.longitude != null) {
        formData.append('latitude', String(profile.latitude));
        formData.append('longitude', String(profile.longitude));
      }

      // 3. Adjuntar los archivos (si se han seleccionado)
      if (avatarFile) {
        formData.append('avatar', avatarFile);
      }
      if (logoFile) {
        formData.append('logo', logoFile);
      }

      // 4. Enviar usando multipart/form-data
      const response = await api.put('/doctor/me', formData, {
        headers: {
          'Content-Type': 'multipart/form-data'
        }
      });

      if (response.data) {
        setMessage({ type: 'success', text: '¡Cambios e imágenes guardados con éxito!' });

        const data = response.data;
        if (data.fullName) {
          setDoctorName(data.fullName);
        }
        // 5. Mapear las URLs que te regresa Go y limpiar la memoria intermedia
        setProfile(prev => ({
          ...prev,
          avatarUrl: data.avatarUrl || prev.avatarUrl,
          logoUrl: data.logoUrl || prev.logoUrl,
          publicListed: data.publicListed,
          publicBio: data.publicBio,
          publicSlug: data.publicSlug || prev.publicSlug,
          latitude: data.latitude ?? prev.latitude,
          longitude: data.longitude ?? prev.longitude
        }));

        if (data.avatarUrl) setAvatarPreview(toAbsoluteFileUrl(data.avatarUrl));
        if (data.logoUrl) setLogoPreview(toAbsoluteFileUrl(data.logoUrl));

        setPopupConfig({
          isOpen: true,
          type: 'success',
          title: '¡Guardado con Éxito!',
          message: 'La información de tu perfil se ha actualizado correctamente.'
        });

        setAvatarFile(null);
        setLogoFile(null);
      }
      setMessage({ type: 'success', text: 'Perfil actualizado correctamente.' });
    } catch (err: unknown) {
      console.error(err);
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error de Servidor',
        message: 'No se pudieron guardar los cambios. Inténtalo de nuevo.'
      });
      setMessage({ type: 'error', text: getErrorMessage(err, 'Error al guardar los cambios.') });
    } finally {
      setIsSaving(false);
    }
  };

  if (isLoading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '60vh', color: 'var(--color-primary)' }}>
        <div className="spinner-border" role="status"></div>
      </div>
    );
  }

  const completionPercentage = calculateCompletion();

  
  return (
    <div className="doctor-profile-container">
      {/* Encabezado */}
      <div className="profile-header">
        <h1>Mi Perfil Médico</h1>
        <p>Configura tu identidad profesional y los datos fiscales de tu consultorio.</p>
      </div>

      {message && (
        <div className={`profile-alert alert-${message.type}`} style={{
          padding: '12px 16px', borderRadius: '8px', marginBottom: '24px', fontSize: '14px', fontWeight: 500,
          backgroundColor: message.type === 'success' ? 'var(--color-success-bg)' : 'var(--color-danger-bg)',
          color: message.type === 'success' ? 'var(--color-success)' : 'var(--color-danger)',
          border: `1px solid ${message.type === 'success' ? 'var(--color-success)' : 'var(--color-danger-border)'}`
        }}>
          {message.type === 'success' ? '✓ ' : '⚠️ '} {message.text}
        </div>
      )}

      {/* 📊 SECCIÓN DE CARD CONTADORES (IDÉNTICO AL DASHBOARD DE CITAS) */}
      <div className="profile-dashboard-summary">
        {/* Tarjeta de Identidad de Marca */}
        <div className="summary-card brand-card">
          <div className="card-icon-wrapper" style={{ backgroundColor: 'rgba(255,255,255,0.15)', color: '#ffffff' }}>
            {avatarPreview ? (
              <img src={avatarPreview} alt="Mini Avatar" />
            ) : (
              <span className="material-icons-outlined">account_circle</span>
            )}
          </div>
          <div className="card-meta">
            <span className="meta-title">Médico Titular</span>
            <span className="meta-subtitle">{profile.fullName || 'No Configurado'}</span>
            <span className="badge-verified">🛡️ Cédula Oficial: {profile.licenseNumber}</span>
          </div>
        </div>

        {/* Tarjeta de Especialidad */}
        <div className="summary-card">
          <div className="card-icon-wrapper">
            <span className="material-icons-outlined">medical_services</span>
          </div>
          <div className="card-meta">
            <span className="meta-title">Especialidad Clínica</span>
            <span className="meta-subtitle">{profile.medicalSpecialty || 'Sin asignar'}</span>
          </div>
        </div>

        {/* Tarjeta de Progreso de Configuración */}
        <div className="summary-card">
          <div className="progress-container">
            <div className="progress-info">
              <span>Completado de Expediente</span>
              <span className="pct">{completionPercentage}%</span>
            </div>
            <div className="progress-track">
              <div className="progress-bar" style={{ width: `${completionPercentage}%` }}></div>
            </div>
          </div>
        </div>
      </div>

      {/* Formulario Estructurado */}
      <form onSubmit={handleSubmit}>
        
        {/* FOTOS / MULTIMEDIA */}
        <div className="profile-form-section">
          <div className="media-upload-wrapper">
            <div className="upload-box">
              <span className="box-title">Foto de Perfil</span>
              <div className="avatar-circle" onClick={() => avatarInputRef.current?.click()}>
                {avatarPreview ? <img src={avatarPreview} alt="Avatar" /> : <span className="material-icons-outlined" style={{color:'var(--color-secondary)', fontSize: '28px'}}>add_a_photo</span>}
              </div>
              <input type="file" ref={avatarInputRef} accept="image/*" style={{ display: 'none' }} onChange={(e) => handleFileChange(e, 'avatar')} />
              <p className="upload-hint">Formatos JPG o PNG.</p>
            </div>

            <div className="upload-box">
              <span className="box-title">Logotipo Corporativo (Recetas)</span>
              <div className="logo-rectangle" onClick={() => logoInputRef.current?.click()}>
                {logoPreview ? <img src={logoPreview} alt="Logo" /> : <span className="material-icons-outlined" style={{color:'var(--color-secondary)', fontSize: '28px'}}>upload_file</span>}
              </div>
              <input type="file" ref={logoInputRef} accept="image/*" style={{ display: 'none' }} onChange={(e) => handleFileChange(e, 'logo')} />
              <p className="upload-hint">Preferiblemente PNG transparente.</p>
            </div>
          </div>
        </div>

        {/* DATOS PROTEGIDOS */}
        <div className="profile-form-section section-readonly">
          <div className="section-title">
            <h3>Información Verificada e Institucional</h3>
            <p>Datos validados ante el Registro de Profesionistas de México. No modificables en consulta.</p>
          </div>
          <div className="grid-layout">
            <div className="form-group">
              <label>Cédula Profesional</label>
              <input type="text" value={profile.licenseNumber} readOnly />
            </div>
            <div className="form-group">
              <label>RFC Médico</label>
              <input type="text" value={profile.rfc} readOnly />
            </div>
            <div className="form-group full-width">
              <label>CURP</label>
              <input type="text" value={profile.curp} readOnly />
            </div>
          </div>
        </div>

        {/* DATOS CONFIGURABLES */}
        <div className="profile-form-section">
          <div className="section-title">
            <h3>Configuración del Consultorio y Datos Públicos</h3>
            <p>Esta información se usará para el agendamiento de citas y generación de recetas electrónicas.</p>
          </div>
          
          <div className="grid-layout">
            {/* 📝 ACERCA DE MÍ */}
            <div className="form-group full-width">
              <label>Acerca de mí / Resumen Profesional</label>
              <textarea 
                name="resume" 
                value={profile.resume} 
                onChange={handleInputChange} 
                rows={4} 
                placeholder="Escribe una breve descripción sobre tu trayectoria médica, enfoque clínico o filosofía de atención médica..." 
              />
            </div>

            <div className="form-group">
              <label>Nombre Completo</label>
              <input type="text" name="fullName" value={profile.fullName} onChange={handleInputChange} required />
            </div>

            <div className="form-group">
              <label>Especialidad Médica</label>
              <input type="text" name="medicalSpecialty" value={profile.medicalSpecialty} onChange={handleInputChange} />
            </div>

            <div className="form-group">
              <label>Correo Electrónico</label>
              <input type="email" name="email" value={profile.email} onChange={handleInputChange} required />
            </div>

            <div className="form-group">
              <label>Teléfono de Contacto</label>
              <input type="tel" name="phone" value={profile.phone} onChange={handleInputChange} />
            </div>

            <div className="form-group full-width">
              <label>Institución Universitaria de Egreso</label>
              <input type="text" name="university" value={profile.university} onChange={handleInputChange} />
            </div>

            <div className="form-group full-width">
              <label>Dirección Física del Consultorio</label>
              <input type="text" name="address" value={profile.address} onChange={handleInputChange} />
            </div>

            <div className="form-group full-width">
              <label>Ubicación en el Mapa</label>
              <LocationPicker
                address={profile.address}
                latitude={profile.latitude ?? null}
                longitude={profile.longitude ?? null}
                onChange={handleLocationChange}
              />
            </div>

            <div className="form-group full-width">
              <label>Leyenda Personalizada para Recetas (COFEPRIS)</label>
              <textarea name="recipeLegend" value={profile.recipeLegend} onChange={handleInputChange} rows={2} />
            </div>
          </div>
        </div>

        {/* DIRECTORIO PÚBLICO */}
        <div className="profile-form-section">
          <div className="section-title">
            <h3>Directorio Público de ProPatient</h3>
            <p>Si lo activas, tu perfil aparece en el directorio público de doctores y pacientes nuevos podrán encontrarte y solicitar una cita en línea, sin necesidad de crear una cuenta.</p>
          </div>
          <div className="public-listing-toggle">
            <label className="toggle-checkbox">
              <input type="checkbox" checked={!!profile.publicListed} onChange={handlePublicListedChange} />
              <span>Aparecer en el directorio público de doctores</span>
            </label>
          </div>

          {profile.publicListed && (
            <div className="grid-layout">
              <div className="form-group full-width">
                <label>Biografía pública (se muestra en tu perfil y en el directorio)</label>
                <textarea
                  name="publicBio"
                  value={profile.publicBio || ''}
                  onChange={handleInputChange}
                  rows={3}
                  placeholder="Cuéntales a los pacientes sobre tu experiencia, enfoque de atención y lo que pueden esperar de una consulta contigo..."
                />
              </div>
              {profile.publicSlug && (
                <div className="form-group full-width">
                  <label>Tu enlace público</label>
                  <input type="text" readOnly value={`${window.location.origin}/dr/${profile.publicSlug}`} onClick={(e) => (e.target as HTMLInputElement).select()} />
                </div>
              )}

              <div className="form-group">
                <label>Facebook</label>
                <input type="url" name="facebookUrl" placeholder="https://facebook.com/tu-página" value={profile.facebookUrl || ''} onChange={handleInputChange} />
              </div>
              <div className="form-group">
                <label>Instagram</label>
                <input type="url" name="instagramUrl" placeholder="https://instagram.com/tu-usuario" value={profile.instagramUrl || ''} onChange={handleInputChange} />
              </div>
              <div className="form-group">
                <label>LinkedIn</label>
                <input type="url" name="linkedinUrl" placeholder="https://linkedin.com/in/tu-perfil" value={profile.linkedinUrl || ''} onChange={handleInputChange} />
              </div>
              <div className="form-group">
                <label>X (Twitter)</label>
                <input type="url" name="twitterUrl" placeholder="https://x.com/tu-usuario" value={profile.twitterUrl || ''} onChange={handleInputChange} />
              </div>
              <div className="form-group">
                <label>TikTok</label>
                <input type="url" name="tiktokUrl" placeholder="https://tiktok.com/@tu-usuario" value={profile.tiktokUrl || ''} onChange={handleInputChange} />
              </div>
              <div className="form-group">
                <label>YouTube</label>
                <input type="url" name="youtubeUrl" placeholder="https://youtube.com/@tu-canal" value={profile.youtubeUrl || ''} onChange={handleInputChange} />
              </div>
              <div className="form-group full-width">
                <label>Sitio web / landing page profesional</label>
                <input type="url" name="websiteUrl" placeholder="https://tu-sitio.com" value={profile.websiteUrl || ''} onChange={handleInputChange} />
              </div>
            </div>
          )}

          {profile.publicListed && <DoctorGallery />}
        </div>

        {/* INTEGRACIÓN CON GOOGLE CALENDAR */}
        <div className="profile-form-section">
          <div className="section-title">
            <h3>Integración con Google Calendar</h3>
            <p>Cuando está conectado, cada cita nueva se agrega a tu Google Calendar; reprogramar o cancelar en PROPatient actualiza el mismo evento.</p>
          </div>
          <div className="calendar-integration-box">
            <div className="calendar-status">
              <span className={`material-icons-outlined ${profile.googleCalendarConnected ? 'connected' : ''}`}>
                {profile.googleCalendarConnected ? 'event_available' : 'event_busy'}
              </span>
              <span>
                {profile.googleCalendarConnected
                  ? 'Tu Google Calendar está conectado.'
                  : 'Aún no has conectado tu Google Calendar.'}
              </span>
            </div>
            {profile.googleCalendarConnected ? (
              <button type="button" className="btn-outline-danger" onClick={handleDisconnectCalendar} disabled={connectingCalendar}>
                {connectingCalendar ? 'Desconectando...' : 'Desconectar'}
              </button>
            ) : (
              <button type="button" className="btn-outline-sm" onClick={handleConnectCalendar} disabled={connectingCalendar}>
                {connectingCalendar ? 'Redirigiendo...' : 'Conectar con Google Calendar'}
              </button>
            )}
          </div>
        </div>

        {/* ACCIONES */}
        <div className="actions-container">
          <button type="submit" className="btn-save-profile" disabled={isSaving}>
            {isSaving ? 'Guardando Cambios...' : 'Guardar Información'}
          </button>
        </div>

      </form>

      {/* Fuera del <form>: son acciones propias, no parte del guardado del perfil. */}
      <div className="profile-form-section">
        <div className="section-title">
          <span className="material-icons-outlined">privacy_tip</span>
          Privacidad y datos
        </div>
        <p style={{ color: 'var(--color-secondary)', fontSize: '13px', marginTop: '-8px', marginBottom: '16px' }}>
          Puedes descargar una copia de todo lo asociado a tu cuenta, o eliminarla por completo.
        </p>
        <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
          <button type="button" className="btn-outline-sm" onClick={handleExportData} disabled={exportingData}>
            {exportingData ? 'Descargando...' : 'Descargar mis datos'}
          </button>
          <button
            type="button"
            className="btn-outline-danger"
            onClick={() => setShowDeleteConfirm(true)}
            disabled={deletingAccount}
          >
            {deletingAccount ? 'Eliminando...' : 'Eliminar mi cuenta'}
          </button>
        </div>
      </div>

    <Popup
        isOpen={popupConfig.isOpen}
        type={popupConfig.type}
        title={popupConfig.title}
        message={popupConfig.message}
        onClose={() => setPopupConfig(prev => ({ ...prev, isOpen: false }))}
      />
    <ConfirmDialog
        isOpen={showDeleteConfirm}
        title="Eliminar mi cuenta"
        message="Tu cuenta se desactivará y ya no podrás iniciar sesión. Los expedientes clínicos se conservan por obligación legal; contáctanos si necesitas su eliminación completa. Si tienes una suscripción activa, cancélala primero desde Suscripción."
        confirmText="Eliminar mi cuenta"
        variant="danger"
        onConfirm={handleDeleteAccount}
        onCancel={() => setShowDeleteConfirm(false)}
      />
    </div>
  );
};