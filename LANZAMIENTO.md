# Checklist de salida a mercado — ProPatient

Este documento junta todo lo que falta para pasar de "funciona en pruebas" a
"tengo doctores y pacientes reales usándolo". Se divide en dos partes:

1. **Configuración** — nada de código, son valores que hay que poner en
   Render (o en otros paneles externos). Todo el código que los usa **ya
   está implementado y probado**; sin estos valores, esas funciones
   simplemente quedan apagadas/degradadas, no rompen el resto de la app.
2. **Recomendaciones** — cosas que no bloquean un lanzamiento pequeño/piloto
   pero conviene resolver antes de escalar.

---

## 🔴 1. Configuración pendiente en Render

Ve a [dashboard.render.com](https://dashboard.render.com) → tu servicio →
**Environment** → agrega cada variable → **Save, rebuild, and deploy**.

### 1.1 Panel de administración (aprobación de cédula profesional)

| Variable | Qué es |
|---|---|
| `SUPERADMIN_USERNAME` | Usuario para entrar a `/admin/login` |
| `SUPERADMIN_PASSWORD` | Contraseña (usa una fuerte — esta cuenta puede aprobar la cédula de cualquier doctor) |

Sin esto, `/admin/login` existe pero no hay ninguna cuenta con la que
entrar — sigues sin poder aprobar doctores nuevos. Se crea automáticamente
la primera vez que el backend arranca con estas dos variables definidas.

### 1.2 Almacenamiento de archivos — ✅ resuelto

| Variable | Qué es |
|---|---|
| `AWS_S3_BUCKET` | Nombre del bucket S3 |
| `AWS_REGION` | ej. `us-east-1` |
| `AWS_ACCESS_KEY_ID` | Credencial de un usuario/rol de AWS con permisos sobre ese bucket |
| `AWS_SECRET_ACCESS_KEY` | Idem |

**Por qué es crítico:** sin estas cuatro, los documentos clínicos, recetas
en PDF, INE de validación, avatar, logo **y ahora también las fotos de la
galería del perfil público** se guardan en el disco local del contenedor.
**El filesystem de un servicio web de Render NO es persistente** — cada vez
que redespliegas (cada `git push`, cada reinicio), esos archivos
desaparecen. Un consultorio real subiendo estudios/recetas/fotos las
perdería en el primer redeploy. **Ya está configurado en Render** — para
confirmar que quedó bien: sube una foto de galería o el avatar del doctor,
haz un redeploy (o espera a que el servicio se reinicie por inactividad si
está en plan free), y verifica que la foto siga apareciendo — si
desapareció, todavía está guardando en disco local en vez de S3.

### 1.3 Correo (Resend) — ✅ resuelto

Dominio `propatient.pro` verificado en Resend (DKIM/MX/SPF/DMARC en
Porkbun) y `RESEND_FROM_EMAIL` configurado con `notificaciones@propatient.pro`
— confirmado con un correo real recibido de punta a punta. No requiere más
trabajo.

### 1.4 WhatsApp (Twilio)

| Variable | Qué es |
|---|---|
| `TWILIO_ACCOUNT_SID` | De tu consola de Twilio |
| `TWILIO_AUTH_TOKEN` | De tu consola de Twilio |
| `TWILIO_WHATSAPP_FROM` | Número de WhatsApp, formato `whatsapp:+52...` |

**Estado actual:** configurado con el número de **sandbox** de pruebas de
Twilio (`+14155238886`), que requiere que cada destinatario mande
`join <código>` una sola vez antes de poder recibir mensajes — no es viable
con pacientes reales que no conocen ese paso. Quedó pendiente resolver el
error 63015 que impedía la entrega incluso después del "join".

**Antes de lanzar con pacientes reales**, necesitas migrar a un número de
**WhatsApp Business API** real:
1. En Twilio Console → Messaging → Senders → WhatsApp senders, solicita un
   número de WhatsApp Business (requiere verificar tu negocio ante Meta —
   puede tardar días).
2. Una vez aprobado, actualiza `TWILIO_WHATSAPP_FROM` con ese número.
3. **Da de alta las plantillas de mensaje** (`TWILIO_TEMPLATE_*`, ver
   `.env.example`) y pide que Meta las apruebe como categoría "utility".
   Esto no es opcional: fuera del sandbox, Meta rechaza cualquier mensaje
   que el negocio inicia (todos los que manda esta app) si no va por una
   plantilla pre-aprobada — el texto libre que se usa hoy solo funciona en
   el sandbox o dentro de una sesión de 24h que el paciente abrió primero.
   Mientras una plantilla no esté aprobada, ese aviso se sigue mandando
   por texto libre (no bloquea el lanzamiento, pero probablemente no
   entregue nada con un número real hasta que la plantilla correspondiente
   esté activa). De paso, "utility" cuesta menos que "marketing".
4. Prueba el flujo completo (solicitud de cita, confirmación, recordatorios,
   **y ahora también la invitación a calificar la consulta**) con un número
   real que no haya hecho "join" a nada — así confirmas que ya no depende
   del sandbox.

Esto es aún más importante que antes: la función de **reseñas de pacientes**
(sección 3) depende 100% de que este WhatsApp llegue de verdad — sin un
número real, el link para calificar nunca le llega al paciente y esa
función queda invisible aunque el código funcione perfecto.

**Nota de costo:** el recordatorio al doctor 60 minutos antes de la cita ya
no va por WhatsApp — se movió a correo (gratis, vía Resend), porque el
doctor ya tiene que entrar a la app para iniciar la consulta y no
justificaba el gasto de ese canal. La invitación a reseña se queda en
WhatsApp a propósito (decisión explícita, no se movió a correo).

### 1.5 Cobros (Stripe)

| Variable | Qué es |
|---|---|
| `STRIPE_SECRET_KEY` | Empieza con `sk_live_...` en producción (NO `sk_test_...`) |
| `STRIPE_PRICE_ID` | El Price ID de tu producto de suscripción, en modo Live |
| `STRIPE_WEBHOOK_SECRET` | Del webhook configurado apuntando a `https://TU-BACKEND/api/billing/webhook`, en modo Live |

**Verifica explícitamente que estés en modo Live, no Test** — es el error
más común al pasar a producción con Stripe (cobrar con llaves de prueba no
mueve dinero real, pero tampoco factura a nadie). En el dashboard de
Stripe, el switch "Test mode" debe estar apagado al generar estas tres
llaves.

### 1.6 reCAPTCHA en el booking público (opcional, recomendado)

| Variable | Qué es |
|---|---|
| `RECAPTCHA_SECRET_KEY` (backend) | Secret key de un sitio reCAPTCHA v3 en [google.com/recaptcha/admin](https://www.google.com/recaptcha/admin) |
| `VITE_RECAPTCHA_SITE_KEY` (frontend) | Site key del mismo registro |

Sin estas dos, el formulario de agendar cita pública sigue funcionando
exactamente igual — solo se pierde esta capa extra de protección contra
spam, encima del rate limiting por IP que ya está activo siempre. Al
registrar el sitio en la consola de Google, usa **reCAPTCHA v3** (no v2) y
agrega tu dominio (`propatient.pro`).

### 1.7 Monitoreo de errores (Sentry, opcional, recomendado)

| Variable | Qué es |
|---|---|
| `SENTRY_DSN` (backend) | DSN de tu proyecto de Sentry para **Go** |
| `VITE_SENTRY_DSN` (frontend) | DSN de tu proyecto de Sentry para **React/JavaScript** |

Sentry necesita un proyecto separado por plataforma (uno para el backend en
Go, otro para el frontend en React) — cada uno tiene su propio DSN, aunque
vivan en la misma cuenta/organización. Los encuentras en tu cuenta de
Sentry en **Settings → Projects → [tu proyecto] → Client Keys (DSN)**; si
todavía no creaste el segundo proyecto, hazlo con "Create Project" y elige
la plataforma correspondiente (Go / React).

Sin estas dos variables, la app funciona exactamente igual — solo no se
reportan los errores a Sentry. El nivel gratuito de Sentry (5,000
errores/mes) alcanza sin problema para el tamaño de un piloto.

### 1.8 Resto de variables (ya deberías tenerlas, pero verifica)

| Variable | Verificar |
|---|---|
| `JWT_SECRET` | Que sea un valor largo y aleatorio (`openssl rand -base64 48`), no el default de `.env.example` |
| `FRONTEND_URL` | URL completa con `https://`, del servicio `propatient-frontend` |
| `VITE_API_URL` (en el servicio frontend) | URL completa con `https://` + `/api`, del servicio `propatient-api` |
| `GOOGLE_CLIENT_ID` | Ya viene con un default funcional; solo cámbialo si usas tu propio proyecto de Google Cloud |

**No definas** `ENABLE_TEST_SEED` en producción — si la agregas, se crea el
doctor de prueba `medico`/`12345` con contraseña pública. Esa variable es
solo para desarrollo local (docker-compose ya la activa ahí por defecto).

---

## 🔴 2. Fuera de Render (paneles externos)

- **Google OAuth — pantalla de consentimiento — ✅ resuelto para el login**:
  publicada en "En producción" desde Google Auth Platform → Público. El
  login normal del doctor (correo/perfil, scope no sensible) ya funciona
  para cualquier usuario, sin límite.
  **Pendiente aparte, no bloquea el lanzamiento**: el scope de Google
  Calendar (`calendar.events`, sensible) todavía necesita pasar por la
  verificación de Google para no mostrar la pantalla de "app no
  verificada" al conectar — ver la guía completa en
  [`GOOGLE-CALENDAR-VERIFICACION.md`](./GOOGLE-CALENDAR-VERIFICACION.md)
  (checklist + guion del video de demostración). La política de privacidad
  ya tiene la sección específica de uso de Calendar que Google exige para
  esa revisión.
- **Dominio propio — ✅ resuelto**: `propatient.pro` conectado en Render
  (frontend en la raíz + `www`, backend en `api.propatient.pro`), DNS en
  Porkbun, SSL emitido, `FRONTEND_URL`/`VITE_API_URL` actualizados, y
  `https://propatient.pro` agregado a los orígenes autorizados de Google
  OAuth. Confirmado funcionando en producción.
- **Confirma que la base de datos no siga en el plan "free" de Render** si
  vas a depender de estos datos en serio: el plan free de Postgres en
  Render tiene almacenamiento limitado y (verifica en tu dashboard) puede
  no incluir backups automáticos ni garantía de retención a largo plazo.
  Sube al plan pagado antes de tener expedientes clínicos reales que no
  puedas permitirte perder.
- **Plan "free" del propio backend**: los servicios web free de Render se
  "duermen" tras un rato sin tráfico, y la primera petición después de eso
  tarda varios segundos (cold start). Aceptable para un piloto chico,
  molesto para uso real — considera subir de plan antes de lanzar en serio.

---

## ✅ 3. Lo que ya está resuelto en código (para referencia)

No requiere trabajo adicional, ya está implementado y probado (backend +
frontend reales, con tests automatizados):

- Rate limiting por IP en login/registro y en agendar cita pública.
- Términos y Condiciones (`/terminos`) y Aviso de Privacidad (`/privacidad`)
  — **siguen siendo placeholders**, ver sección de recomendaciones.
- Panel interno de administración para aprobar/rechazar cédula profesional
  (`/admin/login`), reemplazando la edición manual por SQL.
- El doctor de prueba `medico`/`12345` ya NO se crea en producción por
  defecto.
- Validación de tipo/tamaño real de archivos subidos (documentos clínicos,
  INE, avatar, logo).
- SEO básico (meta tags, Open Graph, `robots.txt`, `sitemap.xml`).
- "Olvidé mi contraseña" para cuentas de personal (staff).
- Exportar/eliminar mis datos desde el Perfil del doctor (derechos ARCO).
- Cobro de suscripción con Stripe (checkout, portal de cliente, webhooks)
  completamente implementado — solo falta la configuración de la sección
  1.5 de arriba.
- Cierre automático nocturno de citas vencidas a "No asistió" (corregido:
  antes dejaba de correr en producción por el modelo de sleep de Render).
- Vista de mes del calendario menos amontonada (máximo 3 citas por día +
  detalle del día completo al seleccionarlo).
- Personal (staff) compartido entre varios doctores/consultorios — una
  misma cuenta puede administrar la agenda de varias clínicas con un solo
  login, eligiendo con cuál consultorio entrar cuando aplica.
- Dominio propio `propatient.pro` y correo con dominio verificado (ver
  arriba).
- **Galería de fotos** (hasta 8) en el perfil público del doctor.
- **Redes sociales / landing page propia** en el perfil público (Facebook,
  Instagram, LinkedIn, X, TikTok, YouTube, sitio web), todo opcional.
- **Reseñas de pacientes por WhatsApp**: al completar una consulta se manda
  automáticamente un link de calificación (1–5 estrellas + comentario) al
  paciente; el doctor aprueba cada reseña antes de que se publique en su
  perfil. Requiere el WhatsApp Business real de la sección 1.4 para
  funcionar con pacientes reales.

---

## 🟡 4. Recomendado antes de escalar (no bloquea un lanzamiento piloto)

- **Revisión legal real** del Aviso de Privacidad y los Términos y
  Condiciones — hoy son placeholders estructurados con un banner de
  "borrador" bien visible, pensados para que un abogado los complete, no
  para publicarse tal cual. Ver el desglose completo en la sección 6.
- **Facturación fiscal (CFDI)** si vas a cobrarle formalmente a
  consultorios en México — Stripe no la genera, necesitarías un PAC
  (proveedor autorizado de certificación) aparte. Ver sección 6.6.
- **Consentimiento explícito de datos de salud — ✅ resuelto**: checkbox
  obligatorio (con link al Aviso de Privacidad) en el formulario de
  agendar cita pública, validado también en el backend.
- **CI automatizado — ✅ resuelto**: GitHub Actions corre `gofmt`/`vet`/
  `go test` (backend) y `vite build`/`vitest` (frontend) en cada push y
  pull request (`.github/workflows/ci.yml`).
- **Monitoreo de errores (Sentry) — ✅ resuelto en código**: backend (Go,
  captura panics/errores por request vía middleware de Gin) y frontend
  (React, `ErrorBoundary` + captura de excepciones no manejadas). Solo
  falta configurar `SENTRY_DSN` (backend) y `VITE_SENTRY_DSN` (frontend)
  con los DSN de tus dos proyectos de Sentry (uno para Go, uno para
  React/JavaScript) — ver sección 1.7.
- **reCAPTCHA v3 — ✅ resuelto en código**: agregado al formulario de cita
  pública, además del rate limiting ya implementado. Solo falta configurar
  `RECAPTCHA_SECRET_KEY`/`VITE_RECAPTCHA_SITE_KEY` (ver sección 1.6) — sin
  esas dos variables, sigue funcionando igual que antes, sin esta capa
  extra.
- Enviar el sitemap (`https://propatient.pro/sitemap.xml`) a Google Search
  Console — el dominio ya está conectado, solo falta darlo de alta ahí.
- **Confirmar que la contraseña de Gmail filtrada** siga revocada (ya lo
  hiciste) y considerar reescribir el historial de git si en algún momento
  el repo se vuelve público.

---

## 🟢 5. Publicidad / marketing — qué ya tienes técnicamente y qué falta decidir

Esto no es código pendiente, es la estrategia de adquisición — pero vale
dejarlo aquí porque varias piezas técnicas ya están listas para soportarla:

**Ya tienes (piezas técnicas):**
- SEO básico (meta tags, Open Graph, `robots.txt`, `sitemap.xml`).
- Perfil público compartible por doctor (`/dr/:slug`) con foto de galería,
  redes sociales y reseñas reales — esto es justo lo que sirve como
  material de prueba social para anunciar cada consultorio piloto.
- Directorio público con mapa (`/doctores`).

**Falta decidir/hacer (no es código):**
- ~~Dar de alta `https://propatient.pro/sitemap.xml` en Google Search Console~~ ✅ resuelto — dominio verificado y sitemap enviado.
- Crear un **Google Business Profile** por consultorio piloto — mejora
  mucho el posicionamiento local en México y es gratis.
- Elegir el canal de adquisición inicial para los primeros consultorios
  piloto (Google Ads local, redes sociales, alianzas directas con
  doctores/clínicas, boca a boca).
- Si vas a anunciar precios públicamente, tener resuelto el tema de CFDI
  (sección 4) antes, para no prometer algo que la facturación no soporta
  todavía.

---

## 🔴 6. Marco legal aplicable — detallado

Esta sección junta todo lo relacionado con la ley, con el detalle suficiente
para llevarlo a una consulta real con un abogado (idealmente uno con
experiencia en salud, no solo en protección de datos genérica — la
combinación de ambas cosas es la que menos abogados dominan de entrada).
**Nada de esto es asesoría legal formal**, es un mapa de qué preguntar.

### 6.1 Protección de datos personales (LFPDPPP)

Ley Federal de Protección de Datos Personales en Posesión de los
Particulares — la ley mexicana que rige todo el manejo de datos de
pacientes, doctores y personal.

- **Aviso de Privacidad completo.** El Artículo 16 de la LFPDPPP exige que
  incluya, como mínimo:
  - Identidad y domicilio del responsable (quién eres tú/tu empresa
    legalmente, no solo "ProPatient").
  - Las finalidades del tratamiento de datos (agendar citas, expediente
    clínico, recetas, facturación, recordatorios, etc. — ya listadas en el
    borrador, falta completarlas formalmente).
  - Opciones para limitar el uso o divulgación de los datos.
  - Los medios para ejercer los derechos ARCO.
  - Las transferencias de datos que se realicen (ver 6.1.2 abajo).
  - El procedimiento para comunicar cambios al aviso.
  - Estado actual: `PrivacyPolicyContent.tsx` tiene la estructura completa
    con las 9 secciones, pero varias siguen con `[Completar: ...]` —
    específicamente el responsable legal, el mecanismo de consentimiento,
    y el procedimiento ARCO necesitan texto real, no placeholder.

- **Datos de salud = "datos personales sensibles" (Art. 3, fracción VI).**
  Esto es más estricto que datos personales normales:
  - Requieren **consentimiento expreso** del titular, no solo el
    consentimiento tácito que basta para datos no sensibles. El checkbox
    obligatorio en el booking público (ya implementado) es un paso fuerte
    en esa dirección, pero confirma con tu abogado si el texto exacto y el
    mecanismo (un checkbox, sin firma) cumple el estándar de "expreso"
    para datos de salud específicamente — a veces se pide un paso
    adicional (ej. reenviar un correo de confirmación).
  - El expediente clínico completo (antecedentes, diagnósticos,
    tratamientos) cae aquí — es el dato más sensible que maneja el
    sistema.

- **Transferencias internacionales de datos (Art. 36-37).** Usas varios
  proveedores con sede fuera de México que procesan datos de tus usuarios:
  - **Google** (login, Calendar, reCAPTCHA) — procesa correo, nombre, y
    (si se conecta Calendar) datos de citas.
  - **Twilio** (WhatsApp) — procesa teléfonos y el contenido de los
    mensajes.
  - **Resend** (correo) — procesa correos y contenido de los emails.
  - **Stripe** (pagos) — procesa datos de facturación del doctor.
  - **AWS S3** (si está configurado) — almacena documentos clínicos,
    recetas, INE.
  - La LFPDPPP exige que estas transferencias se declaren explícitamente
    en el Aviso de Privacidad (ya hay una sección para Google Calendar
    específicamente, agregada esta sesión — falta extenderla a los demás
    proveedores).

- **Consentimiento para reCAPTCHA/Sentry.** Como se agregaron esta sesión,
  técnicamente califican como "transferencia a terceros" (Google, Sentry)
  igual que los de arriba — hay que agregarlos al Aviso de Privacidad
  cuando se activen de verdad (con las llaves configuradas).

- **Derechos ARCO — ✅ ya implementados**: exportar mis datos y eliminar mi
  cuenta, desde el Perfil del doctor.

- **INAI**: si alguna vez hay una brecha de seguridad con datos de salud
  expuestos, la LFPDPPP obliga a notificar al INAI (Instituto Nacional de
  Transparencia, Acceso a la Información y Protección de Datos
  Personales) y a los titulares afectados. Vale la pena tener un plan de
  respuesta a incidentes por escrito antes de que haga falta, no después.

### 6.2 Normativa sanitaria del expediente clínico (NOM-004-SSA3-2012)

Esta es la norma oficial mexicana específica para expedientes clínicos —
distinta de la protección de datos genérica, y la que más fácil se pasa
por alto porque no es "legal" en el sentido usual, es normativa de salud.

- **Retención mínima: 5 años** desde la fecha del último acto médico
  registrado. El sistema ya está alineado con esto (el expediente clínico
  se conserva aunque el doctor elimine su cuenta — ver el mensaje de
  confirmación en `DoctorProfile.tsx`), pero confírmalo formalmente con un
  abogado de salud: hay excepciones (ej. menores de edad, ciertos
  padecimientos) donde el periodo es más largo.
- **Contenido mínimo obligatorio** del expediente: la norma especifica
  campos que debe tener (ficha de identificación, antecedentes,
  exploración física, diagnósticos, notas de evolución, etc.). Vale la
  pena que un especialista compare esto contra las "Notas de consulta
  configurables" del sistema (`SettingsNotes`) para confirmar que un
  doctor pueda cumplir la norma con la configuración que arme.
- **Quién puede acceder**: la norma regula el acceso al expediente
  (paciente, representante legal, autoridad competente). El sistema ya
  aísla expedientes por doctor (un doctor no ve pacientes de otro) y
  bloquea al personal del expediente clínico — vale la pena confirmar que
  esto es suficiente o si se requiere algo más explícito (ej. un log de
  quién accedió a qué expediente y cuándo).

### 6.3 Términos y Condiciones — responsabilidad médica y de plataforma

- **El punto más importante**: dejar clarísimo que ProPatient es una
  **herramienta de gestión de consultorio**, no presta servicios médicos
  ni sustituye el criterio clínico del doctor — la responsabilidad de la
  atención médica es exclusivamente del doctor que la brinda, no de la
  plataforma. Sin esta cláusula bien redactada, ProPatient queda expuesto
  legalmente a reclamos que le corresponden al doctor (mala praxis,
  errores de diagnóstico, etc.).
- **Límites de responsabilidad técnica**: qué pasa si el sistema tiene una
  caída y un doctor pierde acceso a una cita programada, o si un
  recordatorio de WhatsApp no llega — cláusulas estándar de SaaS
  (disponibilidad "best effort", no garantía de 100% uptime salvo que se
  ofrezca un SLA formal).
- **Política de cancelación de la cuenta/suscripción** y qué pasa con los
  datos del doctor si cancela (ver también 6.1: el expediente clínico se
  conserva por NOM-004 aunque la cuenta se cancele).

### 6.4 PROFECO (protección al consumidor)

Aplica porque cobras una **suscripción recurrente** a los doctores:

- Precios claros y sin cargos sorpresa — el flujo de Stripe (checkout +
  portal de cliente) ya muestra el precio antes de cobrar.
- Derecho de cancelación — el portal de cliente de Stripe ya permite
  cancelar, confirma que el texto de tus Términos lo describa
  correctamente (cuándo deja de tener acceso, si hay reembolso parcial).
- Si publicitas precios ("Mejoras... Publicidad" del checklist de
  marketing), deben coincidir exactamente con lo que Stripe cobra.

### 6.5 Menores de edad

Ya implementado: el formulario público de agendar cita
(`PublicDoctorProfile.tsx`) y el alta manual de pacientes
(`PatientForm.tsx`) tienen un checkbox "El paciente es menor de edad".
Al marcarlo:
- Las etiquetas de teléfono/correo cambian para dejar claro que son los
  datos de quien agenda (padre, madre o tutor), no los del menor.
- El texto del checkbox de consentimiento cambia a una declaración
  explícita ("Declaro ser el padre, madre o tutor legal del paciente...")
  en vez del texto genérico.
- Se guarda `Patient.isMinor` en el expediente, visible como una etiqueta
  ("Menor de edad") en `PatientDetail.tsx`, para que quede documentado
  quién autorizó el tratamiento de los datos.

Sigue valiendo la pena confirmar con tu abogado si este nivel de
consentimiento explícito es suficiente para tu caso de uso, o si además
se requiere algo como una copia de identificación del tutor.

### 6.6 Fiscal — CFDI y régimen (ver también la conversación sobre RESICO)

- Emitir CFDI 4.0 por cada cobro de suscripción — Stripe no lo genera
  automáticamente, se necesita un PAC (proveedor autorizado de
  certificación) o la herramienta gratuita del SAT.
- Confirmar el régimen fiscal correcto (RESICO código 626, o Actividad
  Empresarial y Profesional código 612) antes de facturar el primer cobro
  real — pendiente de tu firma electrónica.

### 6.7 Cookies / almacenamiento local

Sin ser tan estricto como el "cookie consent" de la Unión Europea, la
LFPDPPP sí espera transparencia sobre qué se guarda en el navegador del
usuario. El sistema usa `localStorage` (token de sesión, preferencia de
tema) y, si se activan reCAPTCHA/Sentry, esos scripts también dejan sus
propias cookies/almacenamiento. Vale la pena una línea en el Aviso de
Privacidad mencionándolo, aunque sea breve.

---

## Checklist rápido para marcar conforme avances

- [ ] `SUPERADMIN_USERNAME` / `SUPERADMIN_PASSWORD`
- [x] `AWS_S3_BUCKET` / `AWS_REGION` / `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` ✅
- [x] `RESEND_API_KEY` / `RESEND_FROM_EMAIL` (dominio verificado) ✅
- [ ] `TWILIO_*` con número de WhatsApp Business real (no sandbox)
- [ ] `STRIPE_*` en modo Live (no Test)
- [ ] Verificar `JWT_SECRET`, `FRONTEND_URL`, `VITE_API_URL`
- [x] Google OAuth fuera de modo "Testing" (login) ✅
- [ ] Verificación de Google del scope de Calendar (ver `GOOGLE-CALENDAR-VERIFICACION.md`)
- [x] Conectar `propatient.pro` en Render (DNS en Porkbun) y actualizar
      `FRONTEND_URL`/`VITE_API_URL`/Google OAuth con el dominio nuevo ✅
- [ ] Base de datos y backend fuera del plan free de Render
- [ ] Aviso de Privacidad completo y revisado por abogado (LFPDPPP, sección 6.1)
- [ ] Transferencias internacionales de datos declaradas (Google, Twilio, Resend, Stripe, S3 — sección 6.1)
- [ ] Retención de expediente clínico conforme a NOM-004-SSA3-2012 (sección 6.2)
- [ ] Términos y Condiciones con deslinde de responsabilidad médica (sección 6.3)
- [ ] Política de cancelación conforme a PROFECO (sección 6.4)
- [ ] Consentimiento para menores de edad, si aplica (sección 6.5)
- [ ] CFDI/facturación fiscal (sección 6.6)
- [x] Consentimiento explícito de datos de salud en el booking público ✅
- [x] CI automatizado ✅
- [ ] `RECAPTCHA_SECRET_KEY` / `VITE_RECAPTCHA_SITE_KEY` (código ya listo, ver sección 1.6)
- [ ] `SENTRY_DSN` / `VITE_SENTRY_DSN` (código ya listo, ver sección 1.7)
- [x] Sitemap dado de alta en Google Search Console ✅
- [ ] Google Business Profile del/los consultorio(s) piloto
- [ ] Canal de adquisición inicial decidido (Ads / redes / alianzas)
