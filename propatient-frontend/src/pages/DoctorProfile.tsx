import React, { useState, useEffect, useRef } from 'react';
import api from '../api/axios';
import './DoctorProfile.scss';

interface ProfileData {
  rfc: string;
  curp: string;
  cedulaProfesional: string;
  fullName: string;
  email: string;
  telefono: string;
  especialidad: string;
  universidad: string;
  direccionConsultorio: string;
  leyendaReceta: string;
  acercaDeMi: string;
  avatarUrl?: string;
  logoUrl?: string;
}

export const DoctorProfile = () => {
  const [profile, setProfile] = useState<ProfileData>({
    rfc: 'BEMA930812XXX',
    curp: 'BEMA930812HDFXNX01',
    cedulaProfesional: '1282',
    fullName: localStorage.getItem('suggested_fullname') || '',
    email: '',
    telefono: '',
    especialidad: '',
    universidad: '',
    direccionConsultorio: '',
    leyendaReceta: 'Favor de no automedicarse. En caso de presentar reacciones adversas suspenda el medicamento y consulte a su médico.',
    acercaDeMi: ''
  });

  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error', text: string } | null>(null);

  const [avatarPreview, setAvatarPreview] = useState<string | null>(null);
  const [logoPreview, setLogoPreview] = useState<string | null>(null);

  const avatarInputRef = useRef<HTMLInputElement>(null);
  const logoInputRef = useRef<HTMLInputElement>(null);

  const calculateCompletion = () => {
    const fieldsToValidate = [
      profile.fullName, profile.email, profile.telefono, 
      profile.especialidad, profile.universidad, profile.direccionConsultorio,
      profile.acercaDeMi
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
          if (res.data.avatarUrl) setAvatarPreview(res.data.avatarUrl);
          if (res.data.logoUrl) setLogoPreview(res.data.logoUrl);

          profile.fullName = res.data.fullName;
          profile.acercaDeMi = res.data.resume;
          profile.rfc = res.data.rfc;
          profile.curp = res.data.curp;
          profile.cedulaProfesional = res.data.licenseNumber;
          profile.telefono = res.data.phone;
          profile.especialidad = res.data.medicalSpecialty;
          profile.universidad = res.data.university;
          profile.direccionConsultorio = res.data.address;
          profile.leyendaReceta = res.data.recipeLegend;
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

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { name, value } = e.target;
    setProfile(prev => ({ ...prev, [name]: value }));
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>, type: 'avatar' | 'logo') => {
    const file = e.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onloadend = () => {
      if (type === 'avatar') setAvatarPreview(reader.result as string);
      else setLogoPreview(reader.result as string);
    };
    reader.readAsDataURL(file);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSaving(true);
    setMessage(null);

    try {
      await api.put('/doctor/me', profile);
      setMessage({ type: 'success', text: 'Perfil actualizado correctamente.' });
    } catch (err: any) {
      setMessage({ type: 'error', text: err.response?.data?.error || 'Error al guardar los cambios.' });
    } finally {
      setIsSaving(false);
    }
  };

  if (isLoading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '60vh', color: '#005073' }}>
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
          backgroundColor: message.type === 'success' ? '#e6f4ea' : '#fce8e6',
          color: message.type === 'success' ? '#137333' : '#c5221f',
          border: `1px solid ${message.type === 'success' ? '#ceead6' : '#fad2cf'}`
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
            <span className="badge-verified">🛡️ Cédula Oficial: {profile.cedulaProfesional}</span>
          </div>
        </div>

        {/* Tarjeta de Especialidad */}
        <div className="summary-card">
          <div className="card-icon-wrapper">
            <span className="material-icons-outlined">medical_services</span>
          </div>
          <div className="card-meta">
            <span className="meta-title">Especialidad Clínica</span>
            <span className="meta-subtitle">{profile.especialidad || 'Sin asignar'}</span>
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
                {avatarPreview ? <img src={avatarPreview} alt="Avatar" /> : <span className="material-icons-outlined" style={{color:'#b8b6b2', fontSize: '28px'}}>add_a_photo</span>}
              </div>
              <input type="file" ref={avatarInputRef} accept="image/*" style={{ display: 'none' }} onChange={(e) => handleFileChange(e, 'avatar')} />
              <p className="upload-hint">Formatos JPG o PNG.</p>
            </div>

            <div className="upload-box">
              <span className="box-title">Logotipo Corporativo (Recetas)</span>
              <div className="logo-rectangle" onClick={() => logoInputRef.current?.click()}>
                {logoPreview ? <img src={logoPreview} alt="Logo" /> : <span className="material-icons-outlined" style={{color:'#b8b6b2', fontSize: '28px'}}>upload_file</span>}
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
              <input type="text" value={profile.cedulaProfesional} readOnly />
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
                name="acercaDeMi" 
                value={profile.acercaDeMi} 
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
              <input type="text" name="especialidad" value={profile.especialidad} onChange={handleInputChange} />
            </div>

            <div className="form-group">
              <label>Correo Electrónico</label>
              <input type="email" name="email" value={profile.email} onChange={handleInputChange} required />
            </div>

            <div className="form-group">
              <label>Teléfono de Contacto</label>
              <input type="tel" name="telefono" value={profile.telefono} onChange={handleInputChange} />
            </div>

            <div className="form-group full-width">
              <label>Institución Universitaria de Egreso</label>
              <input type="text" name="universidad" value={profile.universidad} onChange={handleInputChange} />
            </div>

            <div className="form-group full-width">
              <label>Dirección Física del Consultorio</label>
              <input type="text" name="direccionConsultorio" value={profile.direccionConsultorio} onChange={handleInputChange} />
            </div>

            <div className="form-group full-width">
              <label>Leyenda Personalizada para Recetas (COFEPRIS)</label>
              <textarea name="leyendaReceta" value={profile.leyendaReceta} onChange={handleInputChange} rows={2} />
            </div>
          </div>
        </div>

        {/* ACCIONES */}
        <div className="actions-container">
          <button type="submit" className="btn-save-profile" disabled={isSaving}>
            {isSaving ? 'Guardando Cambios...' : 'Guardar Información'}
          </button>
        </div>

      </form>
    </div>
  );
};