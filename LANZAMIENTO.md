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

**Plan de clínica (opcional):** además de la suscripción individual de
arriba, crea dos Prices más en Stripe (mismo modo Live) para el plan de
clínica — $5,000 MXN/mes fijo (cubre hasta 5 doctores) y $1,000 MXN/mes
por cantidad (cada doctor adicional):

| Variable | Qué es |
|---|---|
| `STRIPE_CLINIC_BASE_PRICE_ID` | Price ID del plan base de clínica ($5,000 MXN/mes, cantidad fija 1) |
| `STRIPE_CLINIC_EXTRA_DOCTOR_PRICE_ID` | Price ID del cargo por doctor adicional ($1,000 MXN/mes, cantidad variable) |

Sin estas dos, el resto de la app (incluida la suscripción individual)
sigue funcionando igual — solo la pantalla "Mi Clínica" queda
deshabilitada (503).

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

### 1.9 Notificaciones push / PWA (opcional, recomendado)

| Variable | Qué es |
|---|---|
| `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` (backend) | Par de llaves Web Push, se generan una sola vez |
| `VAPID_SUBJECT` (backend) | Un `mailto:` o URL que te identifica ante el navegador |
| `VITE_VAPID_PUBLIC_KEY` (frontend) | La MISMA llave pública de arriba, no una distinta |

La app ahora es instalable (PWA) desde el navegador en celular/tablet/iPad
(ícono en pantalla de inicio, pantalla completa) y, si estas variables
están configuradas, el doctor puede activar un toggle en su Perfil para
recibir una notificación nativa cuando llega una nueva solicitud de cita
pública — alternativa/complemento al aviso por WhatsApp de la sección 1.4.
Cómo generar las llaves: ver la guía completa en `.env.example`.

**Importante para iOS**: Apple solo entrega notificaciones push a una PWA
si el doctor la **instaló a su pantalla de inicio primero** (Safari → ícono
de compartir → "Agregar a inicio") y tiene **iOS 16.4 o más reciente** — no
hay forma de pedir el permiso desde una pestaña normal del navegador en
iPhone/iPad. En Android/Chrome sí funciona desde el navegador sin instalar.

Sin estas variables, la app funciona exactamente igual — el toggle de
notificaciones simplemente no aparece.

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
- **⏳ EN PROCESO — subir la base de datos del plan "free" de Render a uno
  pagado** (decidido, el usuario está haciéndolo desde el dashboard de
  Render). El plan free de Postgres no incluye backups automáticos ni
  garantía de retención a largo plazo — con expedientes clínicos reales
  no es aceptable perderlos. **Pendiente exacto: en cuanto el usuario
  confirme el nombre del plan de pago que eligió, actualizar
  `render.yaml` línea 14 (`plan: free` → el plan real) para que el
  Blueprint no intente regresar la base al plan gratuito en una futura
  sincronización.** El upgrade en sí se hace 100% desde el dashboard de
  Render, sin tocar código ni variables de entorno (salvo que Render pida
  migrar a una base nueva en vez de subir en el mismo lugar — en ese caso
  sí habría que actualizar `DATABASE_URL` en el servicio backend).
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
  — texto completo, específico a como funciona la app (ya no son
  placeholders); **falta la revisión formal de un abogado**, ver sección 6.
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
- **Contenido mínimo del expediente, aplicado en backend (NOM-004)**: una
  cita ya no puede quedar "COMPLETED" sin diagnóstico ni notas de
  consulta con contenido, ni sin un código CIE-10 asociado — antes solo
  era una validación del frontend, fácil de saltarse llamando a la API
  directo. Sigue pendiente que un especialista compare esto contra la
  lista exacta de campos que pide la norma (ver sección 6.2).
- **Validación de cédula profesional, proceso de revisión mejorado**:
  además de la identificación oficial (INE), ahora también se exige subir
  el documento de la cédula en sí, y el panel de revisión
  (`/admin/pendientes`) incluye un enlace directo al buscador público de
  la SEP para cotejar el número contra la fuente oficial. Sigue siendo
  revisión 100% manual — no existe una API oficial de la SEP para
  automatizarla.
- **Firma-imagen en recetas**: el doctor puede subir una imagen de su
  firma manuscrita desde su Perfil (junto a avatar/logo) y se estampa en
  cada receta en PDF. No es una firma criptográfica (tipo e.firma/FIEL) —
  eso quedó evaluado como un proyecto grande aparte, no se ha hecho.

---

## 🟡 4. Recomendado antes de escalar (no bloquea un lanzamiento piloto)

- **Revisión legal real** del Aviso de Privacidad y los Términos y
  Condiciones — el texto ya está completo y específico a como funciona la
  app (sin placeholders ni banner de "borrador", ver sección 3), pero
  sigue sin haber sido revisado ni firmado por un abogado. Ver el
  desglose completo en la sección 6.
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

- **Aviso de Privacidad completo — ✅ resuelto en texto** (verificado
  releyendo el archivo completo esta sesión, ya no hay ningún
  `[Completar...]`). El Artículo 16 de la LFPDPPP exige, como mínimo, y
  así está cubierto en `PrivacyPolicyContent.tsx` (12 secciones):
  - Identidad y domicilio del responsable — sección 1 (responsable con
    nombre real y correo de contacto).
  - Las finalidades del tratamiento de datos — sección 2.
  - Opciones para limitar el uso o divulgación de los datos — sección 3
    (consentimiento expreso) y 11 (ARCO).
  - Los medios para ejercer los derechos ARCO — sección 11, con correo de
    contacto real.
  - Las transferencias de datos que se realicen — sección 8 (Render, S3,
    Stripe, Twilio, Resend, Sentry) y 9 (Google Calendar) — ver la nota
    de abajo sobre Google Login.
  - El procedimiento para comunicar cambios al aviso — sección 12.
  - **Lo que sigue pendiente es la revisión formal de un abogado**, no
    redacción — el texto ya existe y es específico a como funciona la
    app de verdad.

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

- **Transferencias internacionales de datos (Art. 36-37) — ✅ resuelto casi
  del todo.** Usas varios proveedores con sede fuera de México que
  procesan datos de tus usuarios, y ya están declarados en
  `PrivacyPolicyContent.tsx`:
  - **Twilio, Resend, Stripe, AWS S3, Sentry, Render** — sección 8,
    cada uno con su finalidad específica.
  - **Google Calendar** — sección 9, con detalle del scope exacto y cómo
    revocarlo.
  - **✅ resuelto** — se agregó Google (login) a la sección 8, con el
    detalle de qué datos comparte (correo, nombre).

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
- **Contenido mínimo obligatorio** del expediente — ✅ aplicado a nivel
  backend: una cita no puede finalizarse sin diagnóstico o notas clínicas
  con contenido, ni sin código CIE-10 (ver sección 3). La norma también
  especifica campos concretos (ficha de identificación, antecedentes,
  exploración física, diagnósticos, notas de evolución, etc.) — sigue
  pendiente que un especialista compare esa lista exacta contra las
  "Notas de consulta configurables" del sistema (`SettingsNotes`) para
  confirmar que un doctor pueda cumplir la norma completa con la
  configuración que arme.
- **Quién puede acceder**: la norma regula el acceso al expediente
  (paciente, representante legal, autoridad competente). El sistema ya
  aísla expedientes por doctor (un doctor no ve pacientes de otro) y
  bloquea al personal del expediente clínico — vale la pena confirmar que
  esto es suficiente o si se requiere algo más explícito (ej. un log de
  quién accedió a qué expediente y cuándo).

### 6.3 Términos y Condiciones — responsabilidad médica y de plataforma — ✅ resuelto en texto

Verificado releyendo `TermsOfServiceContent.tsx` completo esta sesión:

- **El punto más importante** — sección 4 ("ProPatient no presta
  servicios médicos"): deja clarísimo que la responsabilidad de la
  atención médica es exclusivamente del doctor, con referencia explícita
  a la NOM-004-SSA3-2012.
- **Límites de responsabilidad técnica** — sección 9 ("Disponibilidad del
  servicio") cubre caídas del sistema y fallas de Twilio/Resend/Google
  Calendar; sección 10 ("Limitación de responsabilidad") tiene un límite
  económico concreto (el importe pagado en los tres meses previos al
  reclamo), no un placeholder.
- **Política de cancelación** — sección 7 (precios exactos + cancelación
  vía Stripe) y sección 13 (retención de datos tras cancelar, coincide
  con NOM-004). Pendiente: revisión formal de abogado, no redacción.

### 6.4 PROFECO (protección al consumidor) — ✅ resuelto en lo esencial

- Precios claros y sin cargos sorpresa — sección 7 de Términos, con los
  montos exactos de cada plan.
- Derecho de cancelación — descrito (cancelación vía Stripe, sin
  reembolso de periodos ya cobrados).
- **✅ resuelto** — se agregó el canal de queja (profeco.gob.mx) a la
  sección 7 de Términos. No se incluyó un teléfono porque no pude
  verificarlo en vivo desde este entorno (mismo bloqueo de proxy de
  siempre); confírmalo tú o déjaselo al abogado si quieres agregarlo.
  Derecho de retracto: sigue pendiente de que el abogado confirme si
  aplica a este tipo de servicio.

### 6.5 Menores de edad — ✅ resuelto en texto y en código

Verificado en ambos documentos legales, no solo en el código: el Aviso
de Privacidad (secciones 2 y 3) y los Términos (sección 8) ya mencionan
explícitamente el caso de un paciente menor de edad y el consentimiento
de quien ejerce la patria potestad o tutela — coincide con lo ya
implementado en el formulario público de agendar cita
(`PublicDoctorProfile.tsx`) y el alta manual de pacientes
(`PatientForm.tsx`), que tienen el checkbox "El paciente es menor de
edad". Al marcarlo:
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

### 6.6 Fiscal — CFDI y régimen — ⏳ decidido, falta configurar

**Decisión tomada**: usar la **app de Facturapi para el Marketplace de
Stripe** (https://marketplace.stripe.com/apps/facturapi) en vez de
construir una integración a la medida dentro de ProPatient — se instala
directo en la cuenta de Stripe que ya se usa para cobrar las
suscripciones, sin tocar el backend/frontend de ProPatient. También se
evaluó Facturama y una integración custom contra la API REST de
Facturapi; se descartaron por ahora a favor de la app lista, que da
cumplimiento sin código nuevo que mantener.

**Pasos pendientes (100% fuera del repo, en Stripe/Facturapi):**
1. Tener el **CSD** (Certificado de Sello Digital) del SAT — dos archivos
   (`.cer`/`.key`) + contraseña, distinto de la e.firma (la e.firma se
   usa para tramitarlo, no es el CSD en sí). Sin esto no se puede timbrar
   nada por ningún camino.
2. Confirmar con un contador el régimen fiscal correcto (RESICO código
   626, o Actividad Empresarial y Profesional código 612).
3. Instalar la app desde el Marketplace de Stripe → crear/usar una
   organización en Facturapi → capturar datos fiscales (deben coincidir
   exacto con el SAT) → subir el CSD → contratar la suscripción de la
   Stripe App (pago aparte de la API general, verificar precio en el
   dashboard) → configurar preferencias de facturación automática.
4. Probar primero en Stripe modo **Test** antes de instalar también en
   modo **Live** (las apps de Stripe se instalan por separado en cada
   modo).

### 6.7 Cookies / almacenamiento local

Sin ser tan estricto como el "cookie consent" de la Unión Europea, la
LFPDPPP sí espera transparencia sobre qué se guarda en el navegador del
usuario. El sistema usa `localStorage` (token de sesión, preferencia de
tema) y, si se activan reCAPTCHA/Sentry, esos scripts también dejan sus
propias cookies/almacenamiento. Vale la pena una línea en el Aviso de
Privacidad mencionándolo, aunque sea breve.

---

## Checklist rápido para marcar conforme avances

- [x] `SUPERADMIN_USERNAME` / `SUPERADMIN_PASSWORD` ✅
- [x] `AWS_S3_BUCKET` / `AWS_REGION` / `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` ✅
- [x] `RESEND_API_KEY` / `RESEND_FROM_EMAIL` (dominio verificado) ✅
- [ ] `TWILIO_*` con número de WhatsApp Business real (no sandbox) + `TWILIO_TEMPLATE_*` (ver sección 1.4)
- [ ] `STRIPE_*` en modo Live (no Test)
- [x] Verificar `JWT_SECRET`, `FRONTEND_URL`, `VITE_API_URL` ✅
- [x] Google OAuth fuera de modo "Testing" (login) ✅
- [ ] Verificación de Google del scope de Calendar (ver `GOOGLE-CALENDAR-VERIFICACION.md`)
- [x] Conectar `propatient.pro` en Render (DNS en Porkbun) y actualizar
      `FRONTEND_URL`/`VITE_API_URL`/Google OAuth con el dominio nuevo ✅
- [ ] Base de datos fuera del plan free de Render — ⏳ en proceso, falta
      que el usuario confirme el plan elegido para actualizar
      `render.yaml` (ver sección 2). Backend (servicio web) todavía en
      plan free, sin resolver.
- [x] Aviso de Privacidad completo en texto (LFPDPPP, sección 6.1) ✅ —
      falta solo revisión formal de abogado, no redacción
- [x] Transferencias internacionales de datos declaradas (sección 6.1) ✅ —
      Google Login agregado; el resto ya estaba
- [x] Retención de expediente clínico conforme a NOM-004-SSA3-2012 (sección 6.2) ✅
- [x] Términos y Condiciones con deslinde de responsabilidad médica (sección 6.3) ✅ —
      falta solo revisión formal de abogado, no redacción
- [x] Política de cancelación conforme a PROFECO (sección 6.4) ✅ — canal
      de queja agregado; derecho de retracto sigue pendiente de confirmar
      con abogado
- [x] Consentimiento para menores de edad, si aplica (sección 6.5) ✅
- [ ] CFDI/facturación fiscal (sección 6.6) — ⏳ decidido usar la app de
      Facturapi para Stripe (sin código nuevo); falta CSD del SAT +
      instalarla y configurarla
- [x] Consentimiento explícito de datos de salud en el booking público ✅
- [x] CI automatizado ✅
- [x] `RECAPTCHA_SECRET_KEY` / `VITE_RECAPTCHA_SITE_KEY` ✅
- [x] `SENTRY_DSN` / `VITE_SENTRY_DSN` ✅
- [x] Sitemap dado de alta en Google Search Console ✅
- [x] Contraseña de Gmail filtrada revocada ✅
- [ ] Google Business Profile del/los consultorio(s) piloto
- [ ] Canal de adquisición inicial decidido (Ads / redes / alianzas)
