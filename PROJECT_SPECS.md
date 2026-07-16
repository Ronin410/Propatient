# ProPatient — Especificaciones del Proyecto

> Documento generado a partir del estado real del código en la rama `claude/project-specs-status-wuvi4a` (13/07/2026).
> Resume qué es el sistema, cómo está construido y qué funcionalidad existe hoy.

## 1. Descripción general

ProPatient es un ecosistema web para consultorios médicos que cubre:

- **Autenticación y onboarding** de médicos (registro clásico, login con Google, validación de cédula profesional).
- **Gestión de pacientes** (alta, edición, búsqueda, ficha clínica).
- **Agendado de citas** (calendario, alta de citas, seguimiento del día).
- **Atención/consulta clínica** (notas dinámicas configurables por el médico, historial clínico, generación de recetas en PDF, documentos adjuntos).
- **Perfil del médico** y ajustes personalizados de plantillas de notas.

## 2. Arquitectura y stack

Monorepo con dos aplicaciones independientes orquestadas por Docker Compose:

| Componente | Tecnología |
|---|---|
| Backend | Go + Gin + GORM (`PropatientGo/`) |
| Base de datos | PostgreSQL 15 (Docker) |
| Frontend | React 19 + TypeScript + Vite (`propatient-frontend/`) |
| Estilos | SCSS + Tailwind CSS 4 |
| Autenticación | JWT propio + Google Identity Services (login social) |
| PDF | `pdfmake` (generación de recetas en el navegador) |
| Envío de correo | `net/smtp` (Gmail SMTP) para notificar validación de cédula |

### 2.1 Servicios (docker-compose.yml)

- `db`: Postgres 15, puerto `5432`, DB `propatient`.
- `backend`: Go API, puerto `8095`, variables `DATABASE_URL`, `JWT_SECRET`, `PORT`.
- `frontend`: Vite dev server, puerto `5173`, `VITE_API_URL=http://localhost:8095/api`.

## 3. Backend (`PropatientGo/`)

### 3.1 Estructura

```
PropatientGo/
├── main.go                     # bootstrap: DB, migraciones, seed, rutas, CORS, worker nocturno
├── internal/
│   ├── auth/                   # login clásico, login Google, JWT, middleware, onboarding
│   ├── handlers/                # appointment, patient, doctor, doctor_template, utils
│   ├── models/                  # entidades GORM
│   ├── repository/               # queries de negocio (cierre nocturno de citas)
│   ├── database/                # seed inicial
│   └── workers/                  # cron nocturno de no-shows
└── uploads/                     # archivos subidos (INE, documentos médicos, recetas)
```

### 3.2 Modelo de datos (GORM)

- **Doctor**: usuario médico. Incluye datos de onboarding (`ProfileCompleted`, `CedulaValidated`), datos profesionales (`LicenseNumber`, `RFC`, `CURP`, `University`, `IneDocumentPath`), branding (`AvatarUrl`, `LogoUrl`, `RecipeLegend`), relación N:M con `Patient` vía `doctor_patients`.
- **Patient**: datos personales + relación 1:1 con `MedicalHistory` + relación 1:N con `Appointment`.
- **MedicalHistory**: alergias, antecedentes patológicos/no patológicos/quirúrgicos/heredofamiliares, medicación actual, gineco-obstétricos, hábitos.
- **Appointment**: cita clínica con `Status` (`PENDING`, `NOSHOW`, y estados manejados desde frontend), `RegistrationStatus`, `Diagnosis`, `TreatmentPlan`, `DynamicNotes` (JSONB — notas configurables por el doctor), `RecipePDFPath`, documentos adjuntos.
- **MedicalDocument**: archivo asociado a una cita (puede marcarse como receta/`Prescription`).
- **DoctorTemplate**: plantilla JSONB por doctor que define los apartados de notas dinámicas que usará en consulta.

### 3.3 Endpoints (API REST bajo `/api`)

**Públicos**
- `POST /auth/login` — login usuario/contraseña.
- `POST /auth/register` — alta de doctor.
- `POST /auth/google-login` — login/registro automático vía Google ID Token.

**Protegidos (JWT, middleware `AuthorizeJWT`)**

_Dashboard_
- `GET /dashboard/summary` — resumen de citas del día.
- `GET /dashboard/upcoming` — próximas citas.

_Usuario / onboarding_
- `POST /user/update-profile` — completar datos personales del doctor.
- `POST /user/update-license` — validación de cédula (JSON).
- `POST /user/update-license-full` — validación de cédula con carga de INE (multipart) + envío de correo async.

_Pacientes_
- `GET /patients`, `POST /patients`, `GET /patients/search`, `GET /patients/:id`, `PUT /patients/:id`
- `GET /patients/:id/history`, `PUT /patients/:id/medical-history`, `GET /patients/:id/stats`

_Citas_
- `GET /appointments`, `POST /appointments`, `GET /appointments/:id`, `PUT /appointments/:id`
- `POST /appointments/:id/upload-document`, `PUT /appointments/:id/documents/:docId`
- `POST /appointments/:id/save-recipe-pdf`

_Doctor_
- `GET /doctor/me`, `PUT /doctor/me`
- `GET /doctor/template`, `POST /doctor/template` (plantilla de notas dinámicas)

_Utilidades_
- `GET /utils/specialties`

### 3.4 Procesos automáticos

- **Worker nocturno** (`internal/workers/night_cron.go`): corre en goroutine, calcula el tiempo hasta medianoche y ejecuta `repository.ExecNightClosure`, que marca como `NOSHOW` las citas `PENDING` cuya fecha ya pasó.
- **Seed inicial** (`internal/database/seed.go`): crea un doctor de prueba (`medico` / `12345`) si no existe, solo cuando `ENABLE_TEST_SEED=true` (docker-compose lo activa por defecto en local; nunca se define en Render/producción).

### 3.5 Seguridad

- Contraseñas con `bcrypt`.
- JWT propio (`GenerateToken`) para sesiones, incluso tras login con Google.
- Validación de Google ID Token contra `oauth2.googleapis.com/tokeninfo`.
- CORS restringido a `http://localhost:5173`.

## 4. Frontend (`propatient-frontend/`)

### 4.1 Rutas (`App.tsx`)

| Ruta | Página | Notas |
|---|---|---|
| `/login` | `Login` | pública, incluye botón de Google Identity Services |
| `/registro/perfil` | `CompleteProfile` | onboarding paso 1 (protegida, fuera del layout de dashboard) |
| `/registro/validar-cedula` | `ValidateLicense` | onboarding paso 2 |
| `/inicio` | `AppointmentTracking` | seguimiento de citas del día |
| `/pacientes` | `PatientList` | listado/búsqueda |
| `/pacientes/:id` | `PatientDetail` | ficha del paciente |
| `/pacientes/nuevo`, `/pacientes/editar/:id` | `PatientForm` | alta/edición |
| `/calendar` | `AppointmentCalendar` | vista de calendario |
| `/appointments/new` | `AppointmentForm` | alta de cita |
| `/consulta/:appointmentId` | `ConsultationManager` | pantalla principal de atención clínica (la más grande del proyecto, ~1300 líneas) |
| `/profile` | `DoctorProfile` | perfil del médico |
| `/ajustes-notas` | `SettingsNotes` | configuración de plantillas de notas dinámicas |

Las rutas del sistema principal están envueltas por `OnboardingGuard` (fuerza a completar perfil/cédula antes de usar el dashboard) y `ProtectedRoute` (requiere sesión).

### 4.2 Módulo de Consulta Clínica (`ConsultationManager.tsx`)

Es el corazón funcional del sistema:
- Carga historial y datos del paciente/cita.
- Notas dinámicas (`dynamicNotes`) basadas en una plantilla configurable por el doctor (`doctor_notes_template`, persistida también vía `/doctor/template`).
- Generación de **receta en PDF** con `pdfmake`, selección de qué secciones incluir (`recipeSections`), impresión directa y guardado del PDF en el backend (`save-recipe-pdf`).
- Guardado de diagnóstico, plan de tratamiento y notas dinámicas en la cita.
- Gestión de documentos médicos adjuntos.

### 4.3 Autenticación en frontend

- `AuthContext` maneja sesión/JWT.
- `OnboardingGuard` redirige según `profileCompleted` / `cedulaValidated` del doctor.
- Login social con Google Identity Services (`client_id` embebido en `Login.tsx`).

## 5. Estado actual / avance

- ✅ Autenticación clásica y con Google funcionando end-to-end.
- ✅ Onboarding de doctor (perfil + validación de cédula con carga de INE + correo de notificación).
- ✅ CRUD de pacientes y su historial clínico.
- ✅ Agendado, calendario y seguimiento de citas.
- ✅ Cierre automático nocturno de citas vencidas (no-show).
- ✅ Consulta clínica con notas dinámicas configurables por doctor.
- ✅ Generación, impresión y guardado de recetas en PDF.
- ✅ Carga de documentos médicos por cita.
- ⚠️ Diseño visual unificado entre pantallas (dashboard, login, perfil) en progreso — commits recientes (`Se integra mismos colores en interfaces`, `Cambios en el diseño del dashboard`).
- ⚠️ `ConsultationManager` sigue creciendo (último commit: `Cambios en el consultationmanager`) — candidato a refactor/división de componentes por su tamaño.
- 🚧 Existe una rama paralela `task/task-doctorprofile-login` con trabajo relacionado a perfil del doctor y login, aún no integrada a esta rama.
- 🔧 Archivos de build sueltos en el repo (`PropatientGo/__debug_bin.exe*`, `PropatientGo/tmp/`) — parecen artefactos de desarrollo local (uso de `air` para hot-reload) que convendría revisar si deben ir a `.gitignore`.

## 6. Cómo correr el proyecto

```bash
docker compose up --build
```

- Backend: http://localhost:8095/api (health check en `/api/health`)
- Frontend: http://localhost:5173
- Usuario de prueba: `medico` / `12345`

## 7. Últimos commits (contexto de avance reciente)

```
e231cb2 Cambios en el consultationmanager
f030834 diversos cambios
d3334e2 Actualizacion de appointment tracking
ab3fb0b CAMBIOS PARA EL CAREGADO DE CITAS Y EL PERFIL
36fc8df Se integra mismos colores en interfaces
ccb2301 Cambios en el diseño del dashboard
c5acc23 Envio de correo en login y mejorar vista
389de03 Login con gmail
c810aa9 Proyecto propatient
b41b2d6 Initial commit
```
