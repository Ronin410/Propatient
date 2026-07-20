/// <reference lib="webworker" />
import { precacheAndRoute } from 'workbox-precaching';

declare const self: ServiceWorkerGlobalScope;

// Cacheo estático de la app (JS/CSS/HTML) para que la PWA sea instalable
// y arranque rápido — vite-plugin-pwa inyecta la lista de archivos aquí en
// build time. No se cachea /api ni /uploads: ese contenido siempre debe
// pedirse fresco al backend.
precacheAndRoute(self.__WB_MANIFEST);

interface PushPayload {
  title: string;
  body: string;
  url: string;
}

// Notificación de "nueva solicitud de cita" (ver internal/webpush en el
// backend, función sendPublicBookingPush). Si el payload no trae algo
// reconocible, se muestra un texto genérico en vez de fallar en silencio.
self.addEventListener('push', (event: PushEvent) => {
  let payload: PushPayload = { title: 'ProPatient', body: 'Tienes una notificación nueva.', url: '/inicio' };
  try {
    if (event.data) {
      payload = { ...payload, ...event.data.json() };
    }
  } catch {
    // payload no era JSON válido — se queda con el texto genérico de arriba.
  }

  event.waitUntil(
    self.registration.showNotification(payload.title, {
      body: payload.body,
      icon: '/pwa-192x192.png',
      badge: '/pwa-192x192.png',
      data: { url: payload.url },
    })
  );
});

// Al tocar la notificación: si ya hay una pestaña de la app abierta, la
// enfoca y navega ahí en vez de abrir una ventana nueva.
self.addEventListener('notificationclick', (event: NotificationEvent) => {
  event.notification.close();
  const targetUrl = (event.notification.data?.url as string) || '/inicio';

  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      for (const client of clientList) {
        if ('focus' in client) {
          client.navigate(targetUrl);
          return client.focus();
        }
      }
      return self.clients.openWindow(targetUrl);
    })
  );
});
