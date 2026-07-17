# Verificación de Google — scope de Calendar (`calendar.events`)

Guía para cuando tengas tiempo de completar la verificación de Google para
la función "Conectar con Google Calendar". No bloquea el lanzamiento — hoy
ya funciona con una pantalla de advertencia que el usuario puede pasar
("Avanzado → Ir a propatient (no seguro)"), pero conviene resolverlo antes
de escalar para que se vea profesional.

Se hace desde **Google Auth Platform → Centro de verificación**, dentro del
mismo proyecto de Google Cloud (`static-destiny-500216-r8`).

---

## 1. Checklist de lo que Google va a pedir

- [ ] **Logo de la app** — en "Información de la marca".
- [ ] **URL de inicio** — `https://propatient.pro`.
- [ ] **Política de privacidad pública** — `https://propatient.pro/privacidad`.
      Ya tiene una sección específica sobre el uso de Google Calendar
      (agregada en el código, sección 6 del Aviso de Privacidad) — esto es
      lo que Google revisa primero para un scope de Calendar, sin esto casi
      siempre rechazan la solicitud.
- [ ] **Dominio autorizado** — `propatient.pro` (debería ya estar, del
      trabajo previo de conectar el dominio).
- [ ] **Justificación del scope** — texto corto que Google te pide pegar en
      el formulario. Usa algo como:

  > ProPatient es un sistema de gestión de citas médicas. Cuando un doctor
  > conecta voluntariamente su cuenta de Google Calendar desde su perfil,
  > la aplicación usa el scope `calendar.events` para crear, actualizar y
  > eliminar un evento espejo por cada cita registrada en ProPatient, de
  > forma que el doctor pueda ver su agenda del consultorio directamente en
  > su calendario personal. No se lee ni se modifica ningún otro evento
  > existente en el calendario del doctor. La función es opcional y el
  > doctor puede desconectarla en cualquier momento desde su perfil.

- [ ] **Video de demostración** (YouTube "no listado" es válido) — ver guion abajo.

---

## 2. Guion del video (2–3 minutos)

Grábalo con captura de pantalla (celular o pantalla de computadora, con
audio narrando o subtítulos). El objetivo es que el revisor de Google vea
exactamente cómo y cuándo se usa el permiso.

1. **(0:00–0:20) Contexto rápido**
   Muestra el login de ProPatient como doctor (`propatient.pro/login`) y
   entra al panel. Menciona en una frase que es un sistema de citas
   médicas para consultorios.

2. **(0:20–0:50) Dónde vive la función**
   Ve a **Perfil** (`/profile`) y baja hasta la sección **"Integración con
   Google Calendar"**. Muestra el estado "Aún no has conectado tu Google
   Calendar" y el botón **"Conectar con Google Calendar"**.

3. **(0:50–1:30) La pantalla de consentimiento de Google (lo más importante)**
   Dale clic al botón — esto te manda a la pantalla real de Google pidiendo
   permiso. **Deja que se vea completa la pantalla de consentimiento**,
   incluyendo qué permisos pide ("Ver, editar, compartir y eliminar
   permanentemente todos los calendarios a los que puedes acceder usando
   Google Calendar" — así es como Google describe `calendar.events`).
   Acepta.

4. **(1:30–1:40) Confirmación de vuelta en la app**
   Muestra que regresas a ProPatient y ahora dice "Tu Google Calendar está
   conectado."

5. **(1:40–2:20) El uso real del permiso**
   Ve a **Citas → Agendar Cita**, crea una cita nueva de prueba. Cambia a
   otra pestaña con **Google Calendar abierto** (del mismo doctor) y
   muestra que el evento apareció ahí automáticamente. Si quieres,
   reprograma la cita en ProPatient y muestra que el evento en Google
   Calendar se actualiza solo.

6. **(2:20–2:40) Cómo se desconecta**
   Vuelve a Perfil y muestra el botón **"Desconectar"** — para que quede
   claro que el doctor tiene control total y puede revocarlo cuando quiera.

No hace falta edición ni producción — Google solo necesita ver el flujo
real y completo, sin cortes que oculten la pantalla de consentimiento.

---

## 3. Después de enviarlo

- El estado de la solicitud se puede consultar desde el mismo "Centro de
  verificación".
- Tiempos típicos para un scope **sensible** (no restringido, que es el
  caso de `calendar.events`): 1 a 3 semanas. A veces Google responde con
  preguntas de seguimiento por correo — revisa la bandeja del correo
  asociado al proyecto de Google Cloud.
- Mientras tanto, la función sigue funcionando igual para cualquier
  doctor, solo con la pantalla de advertencia de "app no verificada" antes
  de conectar.
