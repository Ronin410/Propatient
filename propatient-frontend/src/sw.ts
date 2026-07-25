/// <reference lib="webworker" />
import { clientsClaim } from 'workbox-core';
import { precacheAndRoute } from 'workbox-precaching';

declare const self: ServiceWorkerGlobalScope;

// registerType: 'autoUpdate' (ver vite.config.ts) solo actualiza solo con
// la estrategia generateSW — con injectManifest (la que usa este archivo,
// necesaria por los listeners de push/notificationclick de abajo) hay que
// llamar esto a mano. Sin esto, un service worker nuevo se queda
// "esperando" y el navegador sigue sirviendo el bundle viejo cacheado
// hasta que TODAS las pestañas/instancias del sitio se cierren por
// completo — algo que casi nunca pasa en iOS (Safari en segundo plano y
// la PWA agregada a inicio casi nunca se "cierran" de verdad), así que un
// iPhone se queda viendo una versión vieja del sitio mucho más tiempo que
// Android/Chrome.
self.skipWaiting();
clientsClaim();

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

// El lib.dom/webworker de TS todavía no tipa "vibrate" en NotificationOptions
// aunque sí es parte del estándar y Chrome/Edge en Android lo soportan.
type ShowNotificationOptions = NotificationOptions & { vibrate?: number[] };

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
      // badge-192x192.png es una silueta blanca sobre fondo transparente
      // (generada desde el logo), no el ícono a color de arriba: Android
      // usa esta imagen como máscara para el ícono chico de la barra de
      // estado y la repinta con su propio color — pasarle el ícono a
      // color se ve borroso/manchado ahí.
      badge: '/badge-192x192.png',
      vibrate: [200, 100, 200],
      data: { url: payload.url },
      actions: [{ action: 'view', title: 'Ver solicitud' }],
    } as ShowNotificationOptions)
  );
});

// Al tocar la notificación (o su botón "Ver solicitud"): si ya hay una
// pestaña de la app abierta, la enfoca y navega ahí en vez de abrir una
// ventana nueva.
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
