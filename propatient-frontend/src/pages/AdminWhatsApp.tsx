import React, { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import adminApi from '../api/adminAxios';
import { getErrorMessage } from '../utils/errorMessage';
import './AdminWhatsApp.scss';

type Category = 'PATIENT' | 'DOCTOR_SUPPORT';

interface Thread {
  phone: string;
  category: Category;
  lastMessageBody: string;
  lastMessageAt: string;
  lastDirection: 'INBOUND' | 'OUTBOUND';
  unreadCount: number;
  doctorId?: number;
  doctorName?: string;
  patientId?: number;
  patientName?: string;
}

interface Message {
  id: number;
  direction: 'INBOUND' | 'OUTBOUND';
  body: string;
  createdAt: string;
}

const formatDateTime = (iso: string) =>
  new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

// Bandeja de WhatsApp del superadmin: respuestas de pacientes a un aviso
// (que el doctor NUNCA ve, ver el comentario largo en
// models.WhatsAppMessage) y solicitudes de soporte de los propios
// doctores — mismo número de WhatsApp, dos pestañas separadas.
export const AdminWhatsApp: React.FC = () => {
  const navigate = useNavigate();
  const [category, setCategory] = useState<Category>('PATIENT');
  const [threads, setThreads] = useState<Thread[]>([]);
  const [loadingThreads, setLoadingThreads] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [selectedPhone, setSelectedPhone] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [replyText, setReplyText] = useState('');
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);

  const fetchThreads = useCallback(async () => {
    setLoadingThreads(true);
    setError(null);
    try {
      const res = await adminApi.get('/admin/whatsapp/threads', { params: { category } });
      setThreads(res.data || []);
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudieron cargar las conversaciones.'));
    } finally {
      setLoadingThreads(false);
    }
  }, [category]);

  useEffect(() => {
    setSelectedPhone(null);
    setMessages([]);
    fetchThreads();
  }, [fetchThreads]);

  const openThread = async (phone: string) => {
    setSelectedPhone(phone);
    setLoadingMessages(true);
    setSendError(null);
    try {
      const res = await adminApi.get(`/admin/whatsapp/threads/${encodeURIComponent(phone)}/messages`);
      setMessages(res.data || []);
      // El teléfono abierto ya se marcó como leído en el servidor —
      // refrescamos la lista para que el badge de no leídos desaparezca.
      fetchThreads();
    } catch (err: unknown) {
      setError(getErrorMessage(err, 'No se pudo cargar la conversación.'));
    } finally {
      setLoadingMessages(false);
    }
  };

  const handleSend = async () => {
    if (!selectedPhone || !replyText.trim()) return;
    setSending(true);
    setSendError(null);
    try {
      const res = await adminApi.post(`/admin/whatsapp/threads/${encodeURIComponent(selectedPhone)}/messages`, {
        message: replyText.trim(),
      });
      setMessages((prev) => [...prev, res.data]);
      setReplyText('');
    } catch (err: unknown) {
      setSendError(getErrorMessage(err, 'No se pudo mandar el mensaje.'));
    } finally {
      setSending(false);
    }
  };

  const handleLogout = () => {
    localStorage.removeItem('admin_token');
    navigate('/admin/login');
  };

  const selectedThread = threads.find((t) => t.phone === selectedPhone);

  return (
    <div className="admin-doctors-page admin-whatsapp-page">
      <header className="admin-pending-header">
        <h1>
          <span className="material-icons-outlined">forum</span>
          Bandeja de WhatsApp
        </h1>
        <nav className="admin-nav">
          <Link to="/admin/doctores">Doctores</Link>
          <Link to="/admin/pendientes">Cédulas pendientes</Link>
          <Link to="/admin/whatsapp" className="active">WhatsApp</Link>
        </nav>
        <button className="admin-logout-btn" onClick={handleLogout}>
          <span className="material-icons-outlined">logout</span>
          Cerrar sesión
        </button>
      </header>

      <main className="admin-whatsapp-body">
        <div className="whatsapp-tabs">
          <button className={category === 'PATIENT' ? 'active' : ''} onClick={() => setCategory('PATIENT')}>
            <span className="material-icons-outlined">chat</span> Mensajes de Pacientes
          </button>
          <button className={category === 'DOCTOR_SUPPORT' ? 'active' : ''} onClick={() => setCategory('DOCTOR_SUPPORT')}>
            <span className="material-icons-outlined">support_agent</span> Soporte a Doctores
          </button>
        </div>

        {error && <p className="admin-pending-status admin-pending-error">{error}</p>}

        <div className="whatsapp-layout">
          <aside className="whatsapp-thread-list">
            {loadingThreads ? (
              <p className="whatsapp-empty">Cargando...</p>
            ) : threads.length === 0 ? (
              <p className="whatsapp-empty">Sin conversaciones todavía.</p>
            ) : (
              threads.map((t) => (
                <button
                  key={t.phone}
                  className={`thread-item ${selectedPhone === t.phone ? 'active' : ''}`}
                  onClick={() => openThread(t.phone)}
                >
                  <div className="thread-item-top">
                    <strong>{t.patientName || t.doctorName || t.phone}</strong>
                    {t.unreadCount > 0 && <span className="unread-badge">{t.unreadCount}</span>}
                  </div>
                  <span className="thread-phone">{t.phone}</span>
                  {(t.doctorName && t.patientName) && (
                    <span className="thread-context">Cita con {t.doctorName}</span>
                  )}
                  <p className="thread-preview">
                    {t.lastDirection === 'OUTBOUND' ? 'Tú: ' : ''}
                    {t.lastMessageBody}
                  </p>
                  <span className="thread-time">{formatDateTime(t.lastMessageAt)}</span>
                </button>
              ))
            )}
          </aside>

          <section className="whatsapp-conversation">
            {!selectedPhone ? (
              <p className="whatsapp-empty">Selecciona una conversación para verla.</p>
            ) : (
              <>
                <div className="conversation-header">
                  <strong>{selectedThread?.patientName || selectedThread?.doctorName || selectedPhone}</strong>
                  <span>{selectedPhone}</span>
                </div>
                <div className="conversation-messages">
                  {loadingMessages ? (
                    <p className="whatsapp-empty">Cargando...</p>
                  ) : (
                    messages.map((m) => (
                      <div key={m.id} className={`message-bubble ${m.direction === 'OUTBOUND' ? 'outbound' : 'inbound'}`}>
                        <p>{m.body}</p>
                        <span>{formatDateTime(m.createdAt)}</span>
                      </div>
                    ))
                  )}
                </div>
                <div className="conversation-reply">
                  {sendError && <p className="admin-pending-status admin-pending-error">{sendError}</p>}
                  <textarea
                    rows={2}
                    placeholder="Escribe una respuesta..."
                    value={replyText}
                    onChange={(e) => setReplyText(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                        handleSend();
                      }
                    }}
                  />
                  <button disabled={sending || !replyText.trim()} onClick={handleSend}>
                    {sending ? 'Enviando...' : 'Enviar'}
                  </button>
                </div>
              </>
            )}
          </section>
        </div>
      </main>
    </div>
  );
};
