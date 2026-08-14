# ProPatient — App de pacientes en React Native (prompt para ejecutar después)

> **Estado: NO implementar todavía.** Este documento es el prompt/spec
> listo para arrancar el proyecto cuando se decida seguir adelante — pégalo
> completo como instrucción inicial a una sesión de Claude Code cuando
> llegue el momento. Mientras tanto sirve como registro de la decisión y
> del alcance acordado.

## 0. Por qué existe este documento

Los papás/pacientes piden una experiencia "más formal" que instalar la PWA
manualmente (Compartir → Agregar a inicio en iOS, o el menú ⋮ en Android).
Quieren una app real, descargable de App Store / Google Play. Mayoría usa
Android, pero una parte importante usa iOS — hay que cubrir ambas tiendas
desde el día uno, no solo una.

Se decidió **no** extender a nativo el dashboard del doctor (eso se queda
como está, web + PWA) — el problema es puramente del lado del paciente.

## 1. Contexto del proyecto (para quien ejecute esto sin el resto del historial)

ProPatient es un sistema de gestión para consultorios médicos. Monorepo:
- Backend: Go + Gin + GORM, en `PropatientGo/`, desplegado en Render como
  `propatient-api`. Base pública: `https://api.propatient.pro/api`.
- Frontend web: React 19 + TypeScript + Vite, en `propatient-frontend/`,
  desplegado en Render como `propatient-frontend`.
- Ver `PROJECT_SPECS.md` en la raíz del repo para la arquitectura completa
  (modelo de datos, integraciones, cómo se despliega) antes de tocar nada.

**Punto clave: hoy el paciente NO tiene cuenta.** Todo el contacto con él
es por links de un solo uso (perfil público del doctor, subir documentos,
dejar reseña), la mayoría compartidos por WhatsApp (ya integrado con
Twilio: confirmación, recordatorio 24h, seguimiento, solicitud de reseña).
Esta app de React Native reemplaza/complementa esa experiencia web para
quien prefiera algo instalado, sin sustituir WhatsApp como canal — ambos
conviven.

Endpoints públicos ya existentes que esta app debe reutilizar tal cual
(no dupliques lógica de negocio en el cliente, el backend ya la tiene):
```
GET  /api/public/doctors            — directorio (equivalente a /doctores)
GET  /api/public/doctors/:slug      — perfil público de un doctor
POST /api/public/appointments       — agendar cita (sin cuenta)
GET  /api/public/pricing            — precios/promo (informativo, no aplica a la app de pacientes)
GET  /api/public/upload/:token      — info de subida de documentos
POST /api/public/upload/:token      — subir documentos
GET  /api/public/reviews/:token     — invitación a reseña
POST /api/public/reviews/:token     — enviar reseña
```
Revisa `PropatientGo/internal/handlers/public_handler.go` (o el archivo que
implemente cada uno) y `PropatientGo/internal/server/router.go` para el
contrato exacto de cada payload antes de tipar el cliente — no asumas
los campos, léelos del código.

## 2. Alcance de la v1 (MVP) — deliberadamente acotado

**Sí entra:**
1. Directorio de doctores con mapa/lista (mismo criterio que `/doctores`
   en web: nombre, especialidad, ubicación, reseñas).
2. Perfil público del doctor: galería, redes sociales, reseñas, horario,
   botón de agendar.
3. Flujo de agendar cita — **sin login**, igual que hoy en web: nombre,
   teléfono, motivo, checkbox de consentimiento de datos de salud (ya
   existe en el backend, revisa el payload de `POST /api/public/appointments`).
4. Pantalla de confirmación tras agendar.
5. Subir documentos vía el link/token que hoy se manda por WhatsApp (la
   app debe poder abrir ese link como deep link y llevar directo a la
   pantalla de carga, ver sección 4).
6. Dejar reseña vía el link/token de invitación (mismo criterio, deep link).
7. Notificaciones push nativas (ver sección 5 — esto SÍ es trabajo nuevo
   de backend, no existe hoy para clientes nativos).
8. Marca/tema visual consistente con la web (ver sección 6).

**NO entra en v1 (queda para después, no lo construyas sin decisión
explícita):**
- Login/cuenta de paciente, historial de "mis citas" pasadas, ver
  expediente o recetas — el backend no tiene ningún concepto de sesión de
  paciente hoy. Si se pide esto más adelante, es un proyecto aparte de
  autenticación (OTP por SMS/WhatsApp lo más probable) antes de tocar la app.
- Cualquier pantalla del lado del doctor/personal (dashboard, consulta,
  facturación, clínica) — eso se queda exclusivamente en la web.
- Pagos dentro de la app (Stripe ya vive en el flujo del doctor, no del
  paciente).

Si en algún punto el alcance parece estarse expandiendo hacia lo anterior,
para y pregunta antes de seguir — no fue lo que se acordó.

## 3. Stack recomendado

- **Expo** (managed workflow), no React Native CLI puro — evita pelear con
  certificados de Xcode/Android Studio para algo que empieza como "MVP
  simple", y da OTA updates (corregir bugs de UI sin pasar de nuevo por
  revisión de tienda para cada fix menor).
- TypeScript en todo el proyecto (coherencia con el frontend web).
- `expo-router` o `react-navigation` para la navegación (usa el que venga
  por default en el template actual de Expo al momento de ejecutar esto —
  no fijes la versión exacta aquí, puede quedar desactualizada).
- `expo-notifications` para push (ver sección 5) — evita manejar
  certificados APNs/FCM a mano.
- Cliente HTTP simple (fetch o axios) apuntando a
  `https://api.propatient.pro/api`, mismo patrón que
  `propatient-frontend/src/api/axios.ts` (revísalo como referencia de
  manejo de errores).
- Repo: **nuevo repositorio separado** (`propatient-mobile` o similar),
  NO dentro de este monorepo — es una app con su propio ciclo de release
  (versión de tienda, revisiones de Apple/Google) que no debe acoplarse al
  deploy de Render del resto del proyecto.

## 4. Deep links (crítico — sin esto la app no reemplaza los links de WhatsApp)

Hoy los mensajes de WhatsApp llevan URLs web (`https://propatient.pro/dr/:slug`,
`.../public-upload/:token`, `.../resena/:token`). Para que la app se sienta
integrada (no una isla aparte de lo que ya se manda por WhatsApp), configura
**Universal Links (iOS)** y **App Links (Android)** sobre el mismo dominio
`propatient.pro`, de forma que esos links existentes abran la app
instalada en vez del navegador — sin tener que cambiar ni un mensaje de
WhatsApp del lado del backend. Si un usuario no tiene la app instalada,
el link debe seguir funcionando en el navegador exactamente igual que hoy
(fallback normal de Universal/App Links, no requiere nada especial del
backend).

## 5. Notificaciones push — esto SÍ requiere trabajo de backend nuevo

El backend ya tiene push, pero es **Web Push** (VAPID, para navegador/PWA —
ver `PropatientGo/internal/webpush` y `models.PushSubscription`). Una app
nativa de Expo usa un mecanismo distinto (token de Expo Push, que por
debajo habla con APNs/FCM). Hace falta:

1. Backend: modelo nuevo, ej. `ExpoPushToken` (teléfono del paciente +
   token de Expo), paralelo a `PushSubscription`, NO reemplazarlo — la PWA
   del doctor sigue usando el webpush existente sin tocarla.
2. Backend: en los puntos donde hoy se manda WhatsApp al paciente
   (confirmación de cita, recordatorio 24h, seguimiento, solicitud de
   reseña — ver `internal/handlers/appointment_handler.go` y
   `public_handler.go`), agregar el mismo criterio de "mejor esfuerzo" que
   ya existe para WhatsApp/correo: si el teléfono tiene un
   `ExpoPushToken` registrado, mandar también la notificación nativa. No
   quitar WhatsApp — conviven, igual que hoy conviven WhatsApp y correo de
   respaldo.
3. App: al agendar una cita (o al abrir el link de seguimiento), pedir
   permiso de notificaciones y registrar el token de Expo contra ese
   teléfono en el backend.

## 6. Marca / diseño visual

Reutiliza la identidad ya definida en `propatient-frontend/src/index.css`
(no inventes una paleta nueva):
```
Primario:        #005073
Primario oscuro: #002d42
Fondo:            #ffffff / #f8f9fa
Texto:            #495057
Encabezados:      #002d42
Éxito:            #166534   Peligro: #d32f2f   Alerta: #b45309
```
Revisa también el modo oscuro ya implementado en ese mismo archivo (bloque
`[data-theme="dark"]` / `prefers-color-scheme`) y respeta la misma lógica
de tema en la app. Logo e iconos ya existen en
`propatient-frontend/public/` (`pwa-192x192.png`, `pwa-512x512.png`, etc.)
— reutilízalos como base para los icon sets de iOS/Android en vez de
regenerarlos desde cero.

## 7. Distribución — decisiones que requieren al dueño del proyecto, no al agente

Antes de poder subir la primera build a revisión, hace falta que el
usuario (no el agente) resuelva esto — no asumas ni compres nada:
- Cuenta de **Apple Developer Program** ($99 USD/año) a nombre de quién
  corresponda (persona física o la razón social de ProPatient).
- Cuenta de **Google Play Console** ($25 USD, pago único).
- Si más adelante se agrega login social en la app (Google Sign-In, por
  ejemplo), **Apple exige también ofrecer "Sign in with Apple"** como
  alternativa (guideline 4.8 de App Store) — no aplica a la v1 sin login,
  pero queda anotado para cuando se decida agregar cuentas de paciente.
- Nombre exacto de la app en las tiendas, descripción, capturas de
  pantalla — pedir esto al usuario antes de publicar, no inventarlo.

## 8. Definición de "terminado" para la v1

- Las 8 pantallas de la sección 2 funcionando contra el backend real de
  producción (`https://api.propatient.pro/api`), no contra un mock.
- Deep links de las 3 URLs existentes (perfil, upload, reseña) abriendo la
  app en vez del navegador cuando está instalada.
- Notificación push nativa recibida de punta a punta en un dispositivo
  real (Android e iOS) tras agendar una cita de prueba.
- Modo oscuro y claro correctos, siguiendo la paleta de la sección 6.
- Build de producción generada con `eas build` para ambas plataformas,
  lista para subir a revisión (no hace falta que la app ya esté aprobada
  ni publicada para considerar esto "terminado" del lado del código).
