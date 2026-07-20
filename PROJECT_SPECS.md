# ProPatient — Cómo funciona el proyecto

> Documento técnico del estado real del código en la rama
> `claude/project-specs-status-wuvi4a`. Explica arquitectura, modelo de
> datos, integraciones y cómo se despliega. Para el listado de
> funcionalidades desde el punto de vista del usuario, ver
> `FUNCIONALIDADES.md`. Para lo pendiente antes de salir a producción, ver
> `LANZAMIENTO.md`.

## 1. Qué es

ProPatient es un sistema de gestión para consultorios médicos: agenda,
expediente clínico, recetas en PDF, personal compartido entre
consultorios, directorio público con agendado sin cuenta, cobro de
suscripción, y avisos automáticos por correo y WhatsApp.

## 2. Arquitectura y stack

Monorepo con dos aplicaciones independientes, desplegadas como dos
servicios separados en Render.com, con dominio propio `propatient.pro`.

| Componente | Tecnología |
|---|---|
| Backend | Go + Gin + GORM (`PropatientGo/`) |
| Base de datos | PostgreSQL (Render, servicio administrado) |
| Frontend | React 19 + TypeScript + Vite (`propatient-frontend/`) |
| Autenticación de sesión | JWT propio (`golang-jwt/jwt/v5`) |
| Login del doctor | Google Identity Services (OAuth, sin usuario/contraseña propio) |
| Login del personal | Correo/contraseña propio (`bcrypt`) |
| Archivos | AWS S3 (si está configurado) o disco local como respaldo |
| Correo | Resend (API HTTP, no SMTP) |
| WhatsApp | Twilio |
| Cobros | Stripe (checkout + portal de cliente + webhooks) |
| Geocodificación | Nominatim/OpenStreetMap (sin API key) |
| PDF (recetas, historial) | `pdfmake`, generado en el navegador |
| Despliegue | Render.com (Blueprint `render.yaml`), dominio `propatient.pro` |
| Desarrollo local | Docker Compose (Postgres + backend + frontend) |

### 2.1 Por qué está separado así

Cada integración externa (S3, Resend, Twilio, Stripe, Google Calendar) es
**opcional en tiempo de ejecución**: el backend detecta al arrancar si las
variables de entorno correspondientes están definidas y, si no, degrada
esa función sin tumbar el resto de la app (ver sección 6). Esto permite
correr el proyecto completo en local sin ninguna cuenta externa.

## 3. Backend (`PropatientGo/`)

### 3.1 Estructura

```
PropatientGo/
├── main.go                     # bootstrap: DB, migraciones, seed, workers, router
├── internal/
│   ├── auth/                   # login (doctor), JWT, middlewares de rol, onboarding
│   ├── handlers/                # appointment, patient, doctor, staff, admin, billing,
│   │                            #   public, google_calendar, doctor_template, account, utils
│   ├── models/                  # entidades GORM
│   ├── repository/               # queries de negocio (cierre nocturno de citas)
│   ├── database/                # seed inicial (doctor y superadmin de prueba)
│   ├── workers/                  # jobs en segundo plano (cron nocturno, recordatorios)
│   ├── billing/                  # cliente de Stripe + middleware de suscripción
│   ├── storage/                  # cliente de archivos (S3 o disco local)
│   ├── whatsapp/                 # cliente de Twilio
│   ├── googlecalendar/           # cliente OAuth + eventos de Google Calendar
│   ├── geocoding/                # cliente de Nominatim
│   ├── middleware/                # rate limiting
│   ├── server/                    # armado del *gin.Engine (rutas, CORS)
│   └── testutil/                  # helpers compartidos de tests de integración
└── uploads/                     # archivos subidos cuando NO hay S3 configurado
```

### 3.2 Modelo de datos (GORM)

- **Doctor**: cuenta médica. Onboarding (`ProfileCompleted`,
  `CedulaValidated`), datos profesionales (`LicenseNumber`, `RFC`, `CURP`,
  `University`, `IneDocumentPath`), branding (`AvatarUrl`, `LogoUrl`,
  `RecipeLegend`), suscripción (`SubscriptionStatus`, `TrialEndsAt`,
  `StripeCustomerID`, `StripeSubscriptionID`), directorio público
  (`PublicListed`, `PublicBio`, `PublicSlug`, `Latitude`/`Longitude`),
  integración con Google Calendar (`GoogleCalendarRefreshToken`).
  Relación N:M con `Patient` vía `doctor_patients`.
- **Staff**: cuenta de personal (secretaria/asistente). **Ya no
  pertenece a un solo doctor** — se vincula a uno o más consultorios vía
  `DoctorStaff` (ver abajo), para que una misma persona pueda trabajar
  para varias clínicas con un solo login.
- **DoctorStaff**: tabla de unión explícita entre `Doctor` y `Staff`, con
  su propio `Active` — desactivar el acceso desde un consultorio no
  afecta el acceso a los demás.
- **SuperAdmin**: cuenta interna de ProPatient, sin relación con ningún
  consultorio. Su JWT nunca lleva `userId`, así que no puede colarse en
  ninguna ruta de doctor/personal por error.
- **Patient**: datos personales + relación 1:1 con `MedicalHistory` +
  1:N con `Appointment`. Relación N:M con `Doctor` (un paciente puede
  estar vinculado a varios doctores).
- **MedicalHistory**: alergias, antecedentes patológicos/no
  patológicos/quirúrgicos/heredofamiliares, medicación actual,
  gineco-obstétricos, hábitos.
- **Appointment**: cita clínica. `Status` (`PENDING_CONFIRMATION` para una
  solicitud pública sin confirmar, `PENDING`, `COMPLETED`, `CANCELLED`,
  `NOSHOW`), `Diagnosis`, `TreatmentPlan`,
  `DynamicNotes` (JSONB, notas configurables por el doctor),
  `RecipePDFPath`, `FollowUpDate`, `GoogleEventID` (evento espejo),
  timestamps de recordatorios ya enviados (`ReminderSentAt`,
  `DoctorReminderSentAt`) para no reenviar avisos.
- **MedicalDocument**: archivo asociado a una cita (puede marcarse como
  receta vía `Prescription`).
- **DoctorTemplate**: plantilla JSONB por doctor que define los apartados
  de notas dinámicas que usará en consulta.

### 3.3 Autenticación y roles

Todo pasa por JWT (`golang-jwt/jwt/v5`), firmado con `JWT_SECRET`, pero
con **cuatro formas distintas** según quién es:

| Rol (`claims.role`) | Quién | Claims relevantes | Middleware |
|---|---|---|---|
| `MEDICO` | Doctor | `userId` | `AuthorizeJWT` |
| `STAFF` | Personal | `userId` (del doctor activo), `staffId` | `AuthorizeJWT` + `RequireDoctorRole` bloquea rutas solo-doctor |
| `STAFF_SELECT_DOCTOR` | Paso intermedio del login de personal con 2+ consultorios | `staffId`, vive 5 minutos, no da acceso a nada más | validado aparte, no por `AuthorizeJWT` |
| `SUPERADMIN` | Panel interno de ProPatient | `adminId` (nunca `userId`) | `AuthorizeSuperAdminJWT` |

El personal reutiliza el mismo `userId` del doctor dueño del consultorio
elegido en su token — así todos los handlers que ya filtran por
`doctorID` del contexto funcionan igual para doctor y personal, sin
duplicar lógica.

### 3.4 API (`/api`, ver `internal/server/router.go`)

Agrupada por área (no es un listado exhaustivo de cada ruta):

- **`/auth`**: login de doctor (Google), login/invitación/recuperación de
  personal, selección de consultorio (personal con 2+ vínculos), callback
  de Google Calendar.
- **`/admin`**: login y revisión de cédula profesional (solo
  `SUPERADMIN`).
- **`/billing`**: estado de suscripción, checkout y portal de cliente
  (solo doctor).
- **`/public`**: directorio de doctores y agendado de cita sin cuenta.
- **`/user`**: exportar/eliminar mis datos (ARCO), actualizar
  perfil/cédula durante onboarding.
- **`/dashboard`**: resumen del día, próximas citas, estadísticas,
  seguimientos.
- **`/patients`**, **`/appointments`**: CRUD de agenda y pacientes
  (compartido entre doctor y personal, con el contenido clínico
  restringido solo al doctor vía `RequireDoctorRole` ruta por ruta).
- **`/doctor`**: perfil, plantilla de notas, conectar/desconectar Google
  Calendar (solo doctor).
- **`/staff`**: invitar/listar/activar/eliminar personal (solo doctor).
- **`/utils`**: catálogos auxiliares (especialidades).

Rutas protegidas anidan tres capas de middleware: `AuthorizeJWT` (sesión
válida) → `billing.RequireActiveSubscription` (prueba vigente o
suscripción activa) → `RequireDoctorRole` donde aplica (bloquea al
personal). Facturación y ARCO quedan **fuera** de
`RequireActiveSubscription` a propósito: un doctor con la prueba vencida
tiene que poder llegar ahí para pagar o ejercer sus derechos.

### 3.5 Procesos automáticos (`internal/workers/`)

Los tres corren como goroutines con `time.NewTicker` (no un solo
`time.Sleep` largo — ver nota histórica abajo):

- **Cierre nocturno** (`night_cron.go`, cada 15 min): marca como
  `NOSHOW` las citas `PENDING` cuya hora ya pasó.
- **Recordatorio al paciente** (`appointment_reminder.go`, cada 30 min):
  correo + WhatsApp ~24h antes de la cita.
- **Recordatorio al doctor** (`doctor_reminder.go`, cada 10 min): WhatsApp
  ~60 min antes de que empiece la cita.

> Nota histórica: el cierre nocturno originalmente calculaba un único
> `time.Sleep()` hasta medianoche. En el plan free de Render, el proceso
> se duerme por inactividad y ese sleep se perdía silenciosamente — el
> job dejaba de correr en producción. Se corrigió con el patrón de ticker
> (igual que los otros dos workers), que sobrevive a que el proceso se
> reinicie porque cada pasada es independiente.

### 3.6 Seguridad

- Contraseñas con `bcrypt` (doctor no tiene contraseña propia — usa
  Google; personal y súper administrador sí).
- Validación de Google ID Token contra `oauth2.googleapis.com/tokeninfo`,
  verificando `aud` (audience) contra `GOOGLE_CLIENT_ID`.
- Rate limiting por IP (`internal/middleware/ratelimit.go`) en login,
  registro y agendado público — comparte el límite entre las cuatro
  rutas de `/auth` para que alternar entre ellas no lo esquive.
- CORS restringido a los orígenes de `FRONTEND_URL` (soporta varios
  separados por coma), nunca `"*"`.
- Archivos: si S3 está configurado, el bucket es privado y las URLs que
  ve el frontend son firmadas y temporales (20 min) — nadie accede a un
  archivo sin pasar antes por el backend, que ya valida que el doctor sea
  dueño del paciente/cita.
- Migraciones de compatibilidad en `main.go` (índices únicos viejos,
  columnas de `Staff` eliminadas, backfill de periodo de gracia) corren
  en cada arranque, de forma idempotente — no rompen un despliegue que ya
  tenía datos de antes de un cambio de esquema.

## 4. Integraciones externas (todas opcionales en tiempo de ejecución)

El patrón es siempre el mismo: `LoadConfigFromEnv()` lee variables de
entorno, `IsConfigured()` decide si hay lo mínimo para activarla, y si no
la app sigue funcionando con esa función apagada o degradada (nunca
tumba el servidor).

| Integración | Variables | Sin configurar |
|---|---|---|
| Almacenamiento (S3) | `AWS_S3_BUCKET`, `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | Cae a disco local (`./uploads`) — se pierde en cada redeploy de Render |
| Correo (Resend) | `RESEND_API_KEY`, `RESEND_FROM_EMAIL` | No se manda ningún correo (falla rápido, sin bloquear la petición) |
| WhatsApp (Twilio) | `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_WHATSAPP_FROM` | No se manda ningún WhatsApp |
| Cobros (Stripe) | `STRIPE_SECRET_KEY`, `STRIPE_PRICE_ID`, `STRIPE_WEBHOOK_SECRET` | Rutas de facturación devuelven 503; el resto de la app funciona igual |
| Plan de clínica (Stripe) | `STRIPE_CLINIC_BASE_PRICE_ID`, `STRIPE_CLINIC_EXTRA_DOCTOR_PRICE_ID` | Rutas `/api/clinic` devuelven 503; la suscripción individual sigue funcionando igual |
| Google Calendar | `GOOGLE_CALENDAR_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_CALENDAR_REDIRECT_URI` | Botón "Conectar" queda deshabilitado |
| Login del doctor | `GOOGLE_CLIENT_ID` | Trae un default funcional, no es opcional en la práctica |
| Panel SUPERADMIN | `SUPERADMIN_USERNAME`, `SUPERADMIN_PASSWORD` | `/admin/login` existe pero no hay ninguna cuenta con la que entrar |

Detalle de cada una, con pasos de configuración, en `LANZAMIENTO.md`.

## 5. Frontend (`propatient-frontend/`)

### 5.1 Estructura

```
src/
├── pages/          # una página por pantalla (~35 archivos)
├── components/      # compartidos: ConfirmDialog, Popup, guards de ruta, Footer
├── context/         # AuthContext (sesión/rol), ThemeContext (modo oscuro), OnboardingGuard
├── hooks/           # useConsultation (el más grande, toda la lógica de ConsultationManager)
├── api/             # cliente axios con interceptores (auth, 401, 402)
└── utils/           # formateo de fechas, PDFs, sanitización de teléfono, URLs de archivos
```

### 5.2 Rutas principales (`App.tsx`)

- **Públicas**: `/`, `/doctores` (directorio), `/dr/:slug` (perfil
  público + agendar sin cuenta), `/login`, `/staff-login`,
  `/personal/*` (invitación/recuperación de personal), `/admin/login`,
  `/privacidad`, `/terminos`.
- **Onboarding** (doctor nuevo): `/registro/perfil`,
  `/registro/validar-cedula`.
- **Consultorio** (dentro de `DashboardLayout`, requieren sesión):
  `/inicio`, `/pacientes`, `/pacientes/:id`, `/calendar`,
  `/appointments/new`, `/consulta/:appointmentId` (solo doctor),
  `/profile` (solo doctor), `/ajustes-notas` (solo doctor), `/personal`
  (solo doctor), `/billing`.

`OnboardingGuard` fuerza a un doctor nuevo a completar perfil + cédula
antes de usar el resto del sistema. `DoctorOnlyRoute` bloquea en el
frontend las pantallas que el backend ya bloquea para personal (defensa
en profundidad, no la única capa).

### 5.3 `AuthContext`: una sola fuente de verdad para la sesión

Guarda token, rol (`MEDICO`/`STAFF`), estado de onboarding y el nombre a
mostrar en el sidebar — este último vive en el contexto (no solo en
`localStorage`) para que actualizarlo desde Perfil o el onboarding se
refleje al instante en toda la app sin recargar la página.

### 5.4 Cliente HTTP (`api/axios.ts`)

Interceptores centrales:
- Inyecta el JWT en cada petición.
- **401** → limpia la sesión y manda a `/login`.
- **402** (`subscription_required`) → manda a `/billing?locked=1` desde
  cualquier pantalla, sin que cada página tenga que manejarlo por su
  cuenta.

### 5.5 Módulo de Consulta Clínica (`ConsultationManager.tsx` + `useConsultation.ts`)

El corazón funcional del sistema: notas dinámicas configurables por
plantilla, autoguardado de borrador con restauración, bloqueo de
navegación con cambios sin guardar, generación/impresión/guardado de
recetas en PDF (`pdfmake`), carga de documentos adjuntos.

## 6. Despliegue

- **Render.com**, definido en `render.yaml` (Blueprint): base de datos
  Postgres administrada, servicio `propatient-api` (Docker, Go) y
  servicio `propatient-frontend` (build estático de Vite).
- **Dominio propio**: `propatient.pro` (Porkbun), conectado en Render —
  frontend en la raíz + `www`, backend en `api.propatient.pro`. SSL
  emitido automáticamente por Render.
- `VITE_API_URL` se hornea en el **build** del frontend (no en runtime) —
  cambiarlo requiere "Clear build cache & deploy", no solo reiniciar.
- Un solo dominio no puede estar en dos servicios de Render a la vez; por
  eso el backend vive en el subdominio `api.*` en vez de compartir la
  raíz con el frontend.

## 7. Cómo correr el proyecto en local

```bash
docker compose up --build
```

- Backend: http://localhost:8095/api (health check en `/api/health`)
- Frontend: http://localhost:5173
- Usuario de prueba: `medico` / `12345` (solo si `ENABLE_TEST_SEED=true`,
  que `docker-compose.yml` ya activa por defecto — **nunca** se define en
  Render/producción, esa cuenta tiene una contraseña conocida
  públicamente).

Sin ninguna variable de entorno adicional, todas las integraciones
externas (sección 4) quedan degradadas pero el resto de la app funciona
completo.

## 8. Tests

- **Backend**: `go test ./...` (Go, tests de integración reales contra
  Postgres vía `TEST_DATABASE_URL`, usan `internal/testutil` para crear
  doctores/personal/tokens de prueba).
- **Frontend**: `npx vitest run` (Vitest) para lógica pura
  (`utils/`, componentes chicos); `npx tsc -b` para chequeo de tipos
  (`vite build` por sí solo NO valida tipos completos); verificación
  manual en navegador real para flujos completos antes de dar por
  cerrado un cambio de UI.
