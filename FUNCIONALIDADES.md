# Funcionalidades de ProPatient — por pantalla

Este documento describe todo lo que el sistema hace hoy, pantalla por
pantalla y luego por función transversal. Está pensado como referencia
funcional (no técnica) del estado actual del proyecto. Para el detalle de
arquitectura ver `PROJECT_SPECS.md`.

Convención de rutas: las pantallas del consultorio (doctor/personal) viven
dentro de `DashboardLayout` (sidebar + contenido). Las de autenticación,
onboarding y el directorio público son públicas o semi-públicas.

---

## 1. Login del doctor — `/login`

- **Solo login con Google** (Google Identity Services) — no hay
  usuario/contraseña para doctores. El correo de Google es la identidad.
  - Si el correo ya existe como doctor, inicia sesión directo.
  - Si es nuevo, se crea la cuenta automáticamente y lo manda al
    onboarding (`/registro/perfil`), con su nombre sugerido desde Google.
- Enlace a "¿Eres personal del consultorio?" → `/staff-login`.
- Manejo de errores de autenticación con mensajes claros, sin exponer
  detalles técnicos.
- Al autenticar, guarda el JWT y el estado de onboarding, y redirige según
  corresponda (onboarding incompleto → pasos de registro; completo →
  `/inicio`).

## 2. Login del personal — `/staff-login`

Acceso separado para secretarias/asistentes (correo y contraseña
propios, distinto del login de Google del doctor).

- Si la cuenta tiene acceso activo a **un solo** consultorio, entra
  directo, igual que siempre.
- Si tiene acceso activo a **más de un consultorio** (la misma persona
  trabaja para varios doctores de una clínica), aparece una segunda
  pantalla — "¿Con cuál consultorio quieres entrar?" — con un botón por
  cada doctor; al elegir uno, recibe la sesión ya escopada a ese
  consultorio.
- Enlace a "¿Olvidaste tu contraseña?" (recuperación por correo).
- Enlace de vuelta al login del doctor.

## 3. Invitación, recuperación y reseteo de contraseña de personal
`/personal/invitacion/:token`, `/personal/recuperar`, `/personal/restablecer/:token`

- El doctor invita a alguien desde **Gestión de Personal** (sección 13);
  esa persona recibe un correo con un link para crear su contraseña.
- Si el correo invitado **ya tenía cuenta de personal con otro doctor**,
  no se le pide contraseña de nuevo — solo se le vincula al nuevo
  consultorio con la cuenta que ya tiene, y se le avisa por correo que
  ahora puede elegir entre ambos al iniciar sesión.
- "Olvidé mi contraseña": pide el correo, manda un link de un solo uso
  (vence en 1 hora) para definir una contraseña nueva. La respuesta es
  siempre el mismo mensaje genérico exista o no la cuenta, para no
  revelar qué correos están registrados.

## 4. Panel de administración interno (ProPatient) — `/admin/login`

Cuentas internas de ProPatient (`SUPERADMIN`), completamente separadas de
doctores y personal — nunca comparten JWT ni permisos.

- Login propio con usuario/contraseña (se crea la primera cuenta al
  arrancar el backend, vía variables de entorno).
- **Revisión de cédula profesional**: lista de doctores con validación
  "Pendiente", con acceso a su INE y cédula capturados; botones para
  **Aprobar** o **Rechazar**. Reemplaza tener que editar la base de datos
  a mano para dar de alta doctores nuevos.

---

## 5. Onboarding — paso 1: Completar perfil — `/registro/perfil`

Primer paso obligatorio para doctores nuevos.

- Formulario con: nombre completo, especialidad médica, universidad,
  teléfono, fecha de nacimiento (valida mayoría de edad, ≥18 años),
  dirección (opcional), código postal (opcional).
- Si el usuario vino de un login con Google, el nombre se precarga
  automáticamente.
- Validaciones en el formulario antes de enviar.
- Al guardar, avanza automáticamente al siguiente paso.

## 6. Onboarding — paso 2: Validar cédula profesional — `/registro/validar-cedula`

- Formulario con: número de cédula profesional (7–25 caracteres,
  requerido), CURP (formato validado, opcional), RFC (formato validado,
  opcional), costo/tarifa de consulta, y carga de un archivo con el INE
  (identificación oficial, requerido, con validación real de tipo/tamaño).
- Al completar, la solicitud queda "Pendiente" hasta que el súper
  administrador la aprueba desde el panel interno (sección 4); mientras
  tanto, el doctor ya puede usar el resto del sistema con normalidad.

---

## 7. Panel de Control (Dashboard) — `/inicio`

Pantalla principal tras iniciar sesión; primer vistazo del día.

**Encabezado**
- Botón directo "Agendar Cita".

**Tarjetas resumen del día**
- Citas del Día, Pacientes por Atender, Siguiente Paciente (nombre y
  horario, o "línea de espera vacía").

**Métricas del Consultorio**
- Total de Pacientes, Citas de Este Mes (completadas vs. canceladas),
  Tasa de Inasistencia, Próximas Citas en los siguientes 30 días.

**Lista de Atención del Día**
- Tabla con las citas de hoy que siguen activas (excluye completadas,
  canceladas y no-asistidas).
- Por cada cita **Pendiente**, tres acciones: **Iniciar Atención** (pide
  confirmación y navega a la consulta — el personal nunca ve este botón,
  esa pantalla es solo del doctor), **Reprogramar** (modal con
  fecha/hora), **Cancelar** (con confirmación).
- Las citas completadas se muestran con "✓ Atendido".

---

## 8. Listado de Pacientes — `/pacientes`

- Tabla con todos los pacientes vinculados al doctor.
- Buscador en tiempo real por nombre.
- Paginación real del listado general.
- **"Ver Historial"** por paciente → `/pacientes/:id`.
- **"Eliminar"** por paciente: pide confirmación y solo desvincula al
  paciente de ESE doctor — un mismo paciente puede estar vinculado a más
  de un doctor, así que nunca se destruyen datos clínicos.
- Acceso para crear un paciente nuevo.

## 9. Alta / Edición de Paciente — `/pacientes/nuevo`, `/pacientes/editar/:id`

- Campos: nombre, apellido, correo (opcional), teléfono, fecha de
  nacimiento, género.
- Disponible también para el personal (agenda/datos generales, sin acceso
  al expediente clínico).

## 10. Detalle de Paciente — `/pacientes/:id` (solo doctor)

- Datos generales del paciente.
- **Expediente clínico**: alergias, antecedentes patológicos/no
  patológicos/quirúrgicos/heredofamiliares, medicación actual,
  antecedentes gineco-obstétricos, hábitos y estilo de vida — nunca
  visible ni editable por el personal.
- Línea de tiempo de citas del paciente (pasadas y futuras) con su
  estado.
- **"Exportar Historial (PDF)"**: genera y descarga en el navegador un
  PDF con el historial médico completo y la tabla de citas.

---

## 11. Calendario de Citas — `/calendar`

- Vista de **mes** y de **semana**, con navegación (hoy / anterior /
  siguiente) y filtro por estado (Pendientes / Completadas / Canceladas /
  No asistió).
- La vista de mes muestra hasta 3 citas por día para no verse amontonada;
  si hay más, un **"+N más"** abre un panel con **todas** las citas de
  ese día.
- Clic en una cita Pendiente pide confirmar antes de iniciar la consulta;
  clic en una ya atendida/cancelada va directo al expediente del
  paciente.

## 12. Nueva Cita — `/appointments/new`

- Dos modos de selección de paciente: buscar uno existente (autocompletado)
  o registrar uno nuevo directamente desde el mismo formulario.
- Puede abrirse pre-cargada con un paciente específico.
- Campos: fecha y hora (no permite fechas pasadas), motivo de consulta,
  observaciones opcionales.
- Disponible también para el personal.

---

## 13. Consulta Médica — `/consulta/:appointmentId` (solo doctor)

La pantalla más compleja: donde el doctor atiende al paciente y documenta
la consulta. Bloqueada para el personal (ruta protegida) y para
consultas ya cerradas (modo solo-lectura).

- **Notas de consulta dinámicas**: secciones configurables por el propio
  doctor (ver Ajustes de Notas), cada una con su campo de texto —
  adapta la ficha clínica a la especialidad de cada doctor.
- Formulario de datos del paciente editable durante la consulta.
- Carga de archivos/documentos adjuntos (estudios, imágenes, PDFs), con
  vista de los ya subidos.
- **Autoguardado de borrador** mientras el doctor escribe, con
  restauración si la pestaña se cerró a medias.
- **Bloqueo de navegación con cambios sin guardar**: avisa antes de
  perder información si se intenta salir sin guardar.
- **Generar Receta (PDF)**: con los datos del doctor (incluyendo su
  membrete/leyenda configurada en su perfil) y del paciente.
- **Finalizar Consulta**: guarda todo lo capturado, marca la cita como
  completada y genera/guarda la receta.

---

## 14. Perfil del Doctor — `/profile`

- Indicador de porcentaje de perfil completado.
- Datos verificados de solo lectura (cédula, RFC, CURP).
- Datos editables: biografía profesional, nombre, especialidad, correo,
  teléfono, universidad, dirección, y la **leyenda de receta**.
- Carga de foto de perfil y logo del consultorio.
- **Listado público** (opt-in): activar/desactivar que este doctor
  aparezca en el directorio público (`/doctores`), con bio pública y
  ubicación geocodificada automáticamente a partir de su dirección.
- **Google Calendar**: conectar/desconectar la cuenta para que cada cita
  se refleje como evento espejo en su calendario real (ver sección 20).
- **Mis datos (ARCO)**: botón para **exportar todos mis datos** (perfil,
  pacientes, citas, personal) en un archivo descargable, y botón para
  **eliminar mi cuenta** (baja lógica: se conserva el expediente clínico
  por obligación legal, pero la cuenta deja de poder iniciar sesión).

## 15. Ajustes de Notas de Consulta — `/ajustes-notas` (solo doctor)

- Define secciones/campos personalizados de notas: identificador,
  etiqueta visible, texto de ayuda, si es obligatorio o no.
- Agregar y quitar secciones libremente.
- Se usa en la pantalla de Consulta Médica (sección 13).

## 16. Gestión de Personal — `/personal` (solo doctor)

- Lista del personal vinculado a **este** consultorio, con su estado
  (activo/inactivo).
- **Invitar** por nombre y correo:
  - Si el correo es nuevo, se crea la cuenta y se manda invitación para
    crear contraseña.
  - Si el correo **ya tiene cuenta de personal** (con este mismo doctor u
    otro), se vincula directo sin pedirle contraseña de nuevo — este es
    el caso de una clínica donde una misma recepcionista administra la
    agenda de varios doctores con un solo login.
- **Activar/Desactivar**: revoca o restaura el acceso a **este**
  consultorio sin afectar el acceso que esa persona tenga a otros
  consultorios donde también trabaje.
- **Eliminar**: quita el vínculo con este consultorio (nunca borra la
  cuenta compartida si sigue trabajando para otro doctor).

## 17. Facturación / Suscripción — `/billing`

- Todo doctor nuevo arranca con **14 días de prueba gratuita**.
- Estado de la suscripción (en prueba / activa / vencida / cancelada) y
  fecha de vencimiento.
- **Suscribirse**: checkout de pago recurrente (Stripe).
- **Gestionar suscripción**: portal de cliente de Stripe (cambiar método
  de pago, cancelar) sin salir del flujo.
- El personal comparte el estatus de suscripción del doctor que lo
  invitó — si el doctor no paga, el personal también queda bloqueado del
  resto de la app (facturación y derechos ARCO siguen accesibles).

---

## 18. Directorio público de doctores — `/doctores`

Sin necesidad de cuenta ni sesión.

- Mapa interactivo con los doctores que activaron "listado público".
- Ficha resumida por doctor (nombre, especialidad, ubicación).

## 19. Perfil público del doctor — `/dr/:slug`

- Bio, especialidad, ubicación en mapa.
- **Formulario de solicitud de cita sin crear cuenta**: cualquier
  visitante puede pedir una cita; llega al doctor como "Pendiente de
  confirmar" con aviso por correo y WhatsApp. Nunca expone historial
  clínico ni datos de otros pacientes.
- Landing page (`/`) con carrusel de doctores destacados que enlaza aquí.

---

## 20. Integración con Google Calendar (opcional)

- Conectar/desconectar desde el Perfil del doctor (sección 14).
- Cada cita creada, editada o cancelada en ProPatient se refleja como
  evento espejo en el Google Calendar real del doctor.
- Sin conectar, todo funciona igual, solo sin ese espejo.

## 21. Avisos automáticos por correo y WhatsApp

Todo por **mejor esfuerzo**: si el proveedor de correo/WhatsApp no está
configurado, la app sigue funcionando igual, solo no se manda ese aviso
puntual (nunca bloquea al usuario).

- Solicitud de cita pública nueva → aviso al doctor.
- Cita confirmada o rechazada → aviso al paciente.
- Seguimiento marcado → aviso al paciente.
- **Recordatorio al paciente** ~24 horas antes de su cita (revisado cada
  30 min).
- **Recordatorio al doctor** ~60 minutos antes de que empiece una cita
  (revisado cada 10 min).
- Invitación de personal nuevo, aviso de acceso a otro consultorio,
  recuperación de contraseña de personal.
- Correo por Resend, WhatsApp por Twilio.

## 22. Cierre automático nocturno

- Cada 15 minutos, un proceso en segundo plano revisa las citas
  **Pendientes** cuya hora ya pasó y las marca como **"No asistió"**
  automáticamente — no depende de que alguien las marque a mano.

---

## Navegación general — `DashboardLayout`

Estructura compartida por las pantallas del consultorio (secciones 7–17).

- Barra lateral con accesos a: Dashboard, Pacientes, Citas (Calendario),
  Perfil, Ajustes de Notas, Personal, Facturación — algunos ítems
  ocultos para el personal (Perfil, Ajustes de Notas, Personal son solo
  del doctor).
- **Modo oscuro**: interruptor en la barra lateral, aplica a toda la
  interfaz.
- **Cerrar sesión**: pide confirmación y limpia el estado correctamente
  (evita avisos de "cambios sin guardar" fantasma de una consulta
  abierta).

---

## Resumen por función transversal (no ligada a una sola pantalla)

- **Autenticación y sesión**: JWT por rol (doctor, personal, súper
  administrador), login del doctor solo con Google, login del personal
  con correo/contraseña propio, límite de intentos por IP en login/
  registro/agendado público.
- **Personal compartido entre consultorios**: una misma cuenta de
  personal puede trabajar para varios doctores/clínicas, con selección
  de consultorio al iniciar sesión cuando aplica.
- **Multiusuario por doctor**: cada doctor solo ve y gestiona sus propios
  pacientes, citas y personal (aislamiento de datos entre cuentas).
- **Gestión de pacientes**: alta, edición, listado paginado, búsqueda,
  desvinculación, expediente clínico, exportación a PDF.
- **Gestión de citas**: agendar, calendario con filtros y vista de día,
  cancelar, reprogramar, marcar como completada, cierre automático de
  "No asistió".
- **Consulta médica**: notas configurables por especialidad, adjuntos,
  autoguardado, generación de recetas en PDF.
- **Directorio y agendado público**: sin necesidad de cuenta, con avisos
  automáticos al doctor.
- **Notificaciones**: correo (Resend) y WhatsApp (Twilio) en los puntos
  clave del flujo de citas.
- **Cobros**: prueba gratuita de 14 días + suscripción recurrente con
  Stripe (checkout y portal de cliente).
- **Privacidad**: exportar/eliminar mis datos (derechos ARCO), Aviso de
  Privacidad y Términos y Condiciones publicados.
- **Métricas**: panel de estadísticas del consultorio.
- **Personalización**: modo oscuro, membrete/logo propio en recetas.
