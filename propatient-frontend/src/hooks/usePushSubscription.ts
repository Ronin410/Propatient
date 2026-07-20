import { useCallback, useEffect, useState } from 'react';
import api from '../api/axios';

const VAPID_PUBLIC_KEY = import.meta.env.VITE_VAPID_PUBLIC_KEY as string | undefined;

// El navegador exige la llave VAPID en formato Uint8Array (raw), pero el
// backend/entorno la trae en base64url — conversión estándar recomendada
// por la spec de Push API.
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; i++) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

interface UsePushSubscriptionResult {
  // false si el navegador no soporta Push API, o si falta
  // VITE_VAPID_PUBLIC_KEY — en ambos casos el toggle no debe mostrarse.
  supported: boolean;
  subscribed: boolean;
  loading: boolean;
  error: string | null;
  subscribe: () => Promise<void>;
  unsubscribe: () => Promise<void>;
}

// Activa/desactiva las notificaciones push de este navegador/dispositivo
// para el doctor logueado (aviso de nueva solicitud de cita). Guarda la
// suscripción en el backend vía la instancia `api` ya existente (el
// interceptor le agrega el Bearer token solo, ver src/api/axios.ts) — el
// service worker (src/sw.ts) nunca maneja autenticación por su cuenta.
export function usePushSubscription(): UsePushSubscriptionResult {
  const supported =
    !!VAPID_PUBLIC_KEY && typeof navigator !== 'undefined' && 'serviceWorker' in navigator && 'PushManager' in window;
  const [subscribed, setSubscribed] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!supported) {
      setLoading(false);
      return;
    }
    navigator.serviceWorker.ready
      .then((registration) => registration.pushManager.getSubscription())
      .then((existing) => setSubscribed(!!existing))
      .catch(() => setSubscribed(false))
      .finally(() => setLoading(false));
  }, [supported]);

  const subscribe = useCallback(async () => {
    if (!supported || !VAPID_PUBLIC_KEY) return;
    setLoading(true);
    setError(null);
    try {
      const permission = await Notification.requestPermission();
      if (permission !== 'granted') {
        setError('No se concedió el permiso de notificaciones.');
        return;
      }
      const registration = await navigator.serviceWorker.ready;
      const subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        // El cast es por una discrepancia de tipos de lib.dom.d.ts entre
        // Uint8Array<ArrayBufferLike> (lo que devuelve el constructor) y
        // BufferSource (lo que pide subscribe()) — en runtime son
        // exactamente el mismo dato, es solo el tipado de TS.
        applicationServerKey: urlBase64ToUint8Array(VAPID_PUBLIC_KEY) as BufferSource,
      });
      const json = subscription.toJSON();
      await api.post('/doctor/push-subscriptions', {
        endpoint: json.endpoint,
        p256dhKey: json.keys?.p256dh,
        authKey: json.keys?.auth,
      });
      setSubscribed(true);
    } catch {
      setError('No se pudo activar las notificaciones. Intenta de nuevo.');
    } finally {
      setLoading(false);
    }
  }, [supported]);

  const unsubscribe = useCallback(async () => {
    if (!supported) return;
    setLoading(true);
    setError(null);
    try {
      const registration = await navigator.serviceWorker.ready;
      const subscription = await registration.pushManager.getSubscription();
      if (subscription) {
        const endpoint = subscription.endpoint;
        await subscription.unsubscribe();
        await api.delete('/doctor/push-subscriptions', { data: { endpoint } });
      }
      setSubscribed(false);
    } catch {
      setError('No se pudo desactivar las notificaciones. Intenta de nuevo.');
    } finally {
      setLoading(false);
    }
  }, [supported]);

  return { supported, subscribed, loading, error, subscribe, unsubscribe };
}
