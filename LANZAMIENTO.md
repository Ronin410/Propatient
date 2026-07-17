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

### 1.2 Almacenamiento de archivos (⚠️ crítico, fácil de pasar por alto)

| Variable | Qué es |
|---|---|
| `AWS_S3_BUCKET` | Nombre del bucket S3 |
| `AWS_REGION` | ej. `us-east-1` |
| `AWS_ACCESS_KEY_ID` | Credencial de un usuario/rol de AWS con permisos sobre ese bucket |
| `AWS_SECRET_ACCESS_KEY` | Idem |

**Por qué es crítico:** sin estas cuatro, los documentos clínicos, recetas
en PDF, INE de validación, avatar y logo se guardan en el disco local del
contenedor. **El filesystem de un servicio web de Render NO es
persistente** — cada vez que redespliegas (cada `git push`, cada reinicio),
esos archivos desaparecen. Un consultorio real subiendo estudios/recetas
los perdería en el primer redeploy. Configura S3 (o el proveedor
equivalente que prefieras) **antes** de tener documentos clínicos reales
que te importe conservar.

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
3. Prueba el flujo completo (solicitud de cita, confirmación, recordatorios)
   con un número real que no haya hecho "join" a nada — así confirmas que
   ya no depende del sandbox.

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

### 1.6 Resto de variables (ya deberías tenerlas, pero verifica)

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

- **Google OAuth — pantalla de consentimiento**: en
  [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
  revisa si el proyecto sigue en modo "Testing". En ese modo, solo ~100
  correos que agregues manualmente como "test users" pueden loguearse con
  Google, y todos los demás ven una advertencia de "app no verificada".
  Para producción real, hay que pasar el proyecto a "In production"
  (Google puede pedir una revisión si usas scopes sensibles).
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

---

## 🟡 4. Recomendado antes de escalar (no bloquea un lanzamiento piloto)

- **Revisión legal real** del Aviso de Privacidad y los Términos y
  Condiciones — hoy son placeholders estructurados con un banner de
  "borrador" bien visible, pensados para que un abogado los complete, no
  para publicarse tal cual.
- **Facturación fiscal (CFDI)** si vas a cobrarle formalmente a
  consultorios en México — Stripe no la genera, necesitarías un PAC
  (proveedor autorizado de certificación) aparte.
- **Consentimiento explícito** en el formulario de agendar cita pública
  para el tratamiento de datos de salud (checkbox de consentimiento) —
  vale la pena revisar si el formulario actual lo pide.
- **CI automatizado** (ej. GitHub Actions corriendo `go test` y
  `npx vitest run` en cada push) — hoy depende de que alguien los corra a
  mano antes de subir cambios.
- **Monitoreo de errores** (Sentry o similar) — hoy solo te enteras de
  bugs en producción si el usuario te manda una captura del log de Render.
- **reCAPTCHA/hCaptcha** en el formulario de cita pública, además del rate
  limiting ya implementado, si empiezas a ver spam de citas falsas.
- Enviar el sitemap (`https://propatient.pro/sitemap.xml`) a Google Search
  Console — el dominio ya está conectado, solo falta darlo de alta ahí.
- **Confirmar que la contraseña de Gmail filtrada** siga revocada (ya lo
  hiciste) y considerar reescribir el historial de git si en algún momento
  el repo se vuelve público.

---

## Checklist rápido para marcar conforme avances

- [ ] `SUPERADMIN_USERNAME` / `SUPERADMIN_PASSWORD`
- [ ] `AWS_S3_BUCKET` / `AWS_REGION` / `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`
- [x] `RESEND_API_KEY` / `RESEND_FROM_EMAIL` (dominio verificado) ✅
- [ ] `TWILIO_*` con número de WhatsApp Business real (no sandbox)
- [ ] `STRIPE_*` en modo Live (no Test)
- [ ] Verificar `JWT_SECRET`, `FRONTEND_URL`, `VITE_API_URL`
- [ ] Google OAuth fuera de modo "Testing"
- [x] Conectar `propatient.pro` en Render (DNS en Porkbun) y actualizar
      `FRONTEND_URL`/`VITE_API_URL`/Google OAuth con el dominio nuevo ✅
- [ ] Base de datos y backend fuera del plan free de Render
- [ ] Revisión legal de Privacidad/Términos
- [ ] CFDI/facturación fiscal (si aplica)
- [ ] Consentimiento explícito de datos de salud en el booking público
- [ ] CI automatizado
- [ ] Monitoreo de errores (Sentry)
