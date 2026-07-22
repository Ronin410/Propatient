import React, { useEffect, useState } from 'react';
import api from '../api/axios';
import { Popup } from '../components/Popup';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { getErrorMessage } from '../utils/errorMessage';
import type { Staff } from '../types';
import './StaffManagement.scss';

export const StaffManagement: React.FC = () => {
  const [staff, setStaff] = useState<Staff[]>([]);
  const [loading, setLoading] = useState(true);

  const [fullName, setFullName] = useState('');
  const [email, setEmail] = useState('');
  const [inviting, setInviting] = useState(false);

  const [staffToDelete, setStaffToDelete] = useState<Staff | null>(null);

  const [popupConfig, setPopupConfig] = useState({
    isOpen: false,
    type: 'success' as 'success' | 'error',
    title: '',
    message: ''
  });

  const fetchStaff = async () => {
    setLoading(true);
    try {
      const res = await api.get('/staff');
      setStaff(res.data || []);
    } catch {
      // Lista vacía si falla; el usuario puede reintentar recargando.
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStaff();
  }, []);

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    setInviting(true);
    try {
      await api.post('/staff', { fullName, email });
      setFullName('');
      setEmail('');
      setPopupConfig({
        isOpen: true,
        type: 'success',
        title: 'Invitación enviada',
        message: `Le enviamos un correo a ${email} para que cree su contraseña.`
      });
      fetchStaff();
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'No se pudo invitar',
        message: getErrorMessage(err, 'Ocurrió un error al invitar al personal.')
      });
    } finally {
      setInviting(false);
    }
  };

  const handleToggleActive = async (member: Staff) => {
    try {
      await api.put(`/staff/${member.id}/active`, { active: !member.active });
      fetchStaff();
    } catch (err: unknown) {
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error',
        message: getErrorMessage(err, 'No se pudo actualizar el acceso.')
      });
    }
  };

  const handleDeleteConfirmed = async () => {
    if (!staffToDelete) return;
    try {
      await api.delete(`/staff/${staffToDelete.id}`);
      setStaffToDelete(null);
      fetchStaff();
    } catch (err: unknown) {
      setStaffToDelete(null);
      setPopupConfig({
        isOpen: true,
        type: 'error',
        title: 'Error',
        message: getErrorMessage(err, 'No se pudo eliminar la cuenta.')
      });
    }
  };

  const statusLabel = (member: Staff) => {
    if (!member.active) return { text: 'Desactivada', className: 'status-inactive' };
    if (!member.passwordSet) return { text: 'Invitación pendiente', className: 'status-pending' };
    return { text: 'Activa', className: 'status-active' };
  };

  return (
    <div className="staff-management-container">
      <header className="page-header">
        <div>
          <h1>Personal del Consultorio</h1>
          <p className="subtitle">
            Invita a tu secretaria o asistente. Podrán gestionar tu agenda y tus pacientes, sin ver el historial clínico ni tu configuración de cuenta.
          </p>
        </div>
      </header>

      <section className="card invite-card">
        <h3>Invitar a alguien nuevo</h3>
        <form className="invite-form" onSubmit={handleInvite}>
          <div className="form-group">
            <label>Nombre completo</label>
            <input type="text" value={fullName} onChange={(e) => setFullName(e.target.value)} required />
          </div>
          <div className="form-group">
            <label>Correo electrónico</label>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <button type="submit" className="btn-primary" disabled={inviting}>
            {inviting ? 'Enviando...' : 'Enviar invitación'}
          </button>
        </form>
      </section>

      <section className="card staff-list-card">
        <h3>Personal invitado</h3>
        {loading ? (
          <p className="empty-msg">Cargando...</p>
        ) : staff.length === 0 ? (
          <p className="empty-msg">Aún no has invitado a nadie.</p>
        ) : (
          <div className="staff-table-wrapper">
            <table className="staff-table">
              <thead>
                <tr>
                  <th>Nombre</th>
                  <th>Correo</th>
                  <th>Estado</th>
                  <th>Acciones</th>
                </tr>
              </thead>
              <tbody>
                {staff.map((member) => {
                  const status = statusLabel(member);
                  return (
                    <tr key={member.id}>
                      <td data-label="Nombre">{member.fullName}</td>
                      <td data-label="Correo">{member.email}</td>
                      <td data-label="Estado"><span className={`status-badge ${status.className}`}>{status.text}</span></td>
                      <td className="actions-cell" data-label="Acciones">
                        <button className="btn-outline-sm" onClick={() => handleToggleActive(member)}>
                          {member.active ? 'Desactivar' : 'Reactivar'}
                        </button>
                        <button className="btn-outline-sm danger" onClick={() => setStaffToDelete(member)}>
                          Eliminar
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <ConfirmDialog
        isOpen={!!staffToDelete}
        variant="danger"
        title="Eliminar cuenta de personal"
        message={`¿Seguro que quieres eliminar el acceso de ${staffToDelete?.fullName || ''}? No podrá volver a iniciar sesión.`}
        confirmText="Eliminar"
        onConfirm={handleDeleteConfirmed}
        onCancel={() => setStaffToDelete(null)}
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
