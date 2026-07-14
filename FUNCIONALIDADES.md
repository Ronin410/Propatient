# Funcionalidades de ProPatient — por pantalla

Este documento describe, pantalla por pantalla, todo lo que el usuario puede
hacer hoy en ProPatient. Está pensado como referencia funcional (no técnica)
del estado actual del proyecto. Para el detalle de arquitectura ver
`PROJECT_SPECS.md`.

Convención de rutas: todas las pantallas protegidas viven dentro de
`DashboardLayout` (sidebar + contenido). Las de autenticación/onboarding son
públicas o semi-públicas.

---

## 1. Login — `/login`

Pantalla de acceso al sistema.

- Login con usuario y contraseña (formulario clásico).
- Login con Google (botón "Google Identity Services"), que:
  - Si el correo de Google ya existe como doctor, inicia sesión directo.
  - Si es un doctor nuevo, lo crea automáticamente y lo manda al onboarding
    (`/registro/perfil`), sugiriendo su nombre completo tomado de Google.
- Enlace a "Crear cuenta" (registro manual con usuario/contraseña/email).
- Manejo de errores de autenticación con mensajes claros (credenciales
  inválidas, cuenta no verificada, error de red, etc.), usando un helper
  común (`getErrorMessage`) en vez de mostrar errores técnicos crudos.
- Al autenticar, guarda el JWT y el estado de onboarding en `localStorage`
  y redirige según corresponda (onboarding incompleto → pasos de registro;
  completo → `/inicio`).

---

## 2. Onboarding — paso 1: Completar perfil — `/registro/perfil`

Primer paso obligatorio para doctores nuevos (no se puede usar el sistema
sin completarlo).

- Formulario con: nombre completo, especialidad médica, universidad,
  teléfono, fecha de nacimiento (valida mayoría de edad, ≥18 años),
  dirección (opcional), código postal (opcional).
- Si el usuario vino de un login con Google, el campo de nombre completo se
  precarga automáticamente con el nombre sugerido por Google.
- Validaciones en el formulario antes de enviar (campos requeridos, edad
  mínima).
- Al guardar (`POST /user/update-profile`), avanza automáticamente al
  siguiente paso del onboarding: validación de cédula.

## 3. Onboarding — paso 2: Validar cédula profesional — `/registro/validar-cedula`

Segundo y último paso del onboarding.

- Formulario con: número de cédula profesional (7–25 caracteres, requerido),
  CURP (formato validado, opcional), RFC (formato validado, opcional),
  costo/tarifa de consulta, y carga de un archivo con el INE (identificación
  oficial, requerido).
- Envía todo como `multipart/form-data` (`POST /user/update-license-full`).
- Al completar, muestra un aviso y cierra la sesión, mandando al doctor de
  vuelta a `/login` para que inicie sesión ya con el perfil completo (el
  backend queda con la solicitud de validación registrada).

---

## 4. Panel de Control (Dashboard) — `/inicio`

Pantalla principal tras iniciar sesión; primer vistazo del día para el
doctor.

**Encabezado**
- Botón directo "Agendar Cita" que lleva al formulario de nueva cita.

**Tarjetas resumen del día**
- Citas del Día (total agendadas hoy).
- Pacientes por Atender (pendientes de pasar a consulta).
- Siguiente Paciente (nombre y horario del próximo paciente en espera, o
  mensaje de "línea de espera vacía").

**Métricas del Consultorio** (panel de estadísticas agregadas)
- Total de Pacientes vinculados al doctor.
- Citas de Este Mes, con desglose de completadas vs. canceladas.
- Tasa de Inasistencia (histórico de citas marcadas como "no asistió").
- Próximas Citas pendientes en los siguientes 30 días.

**Lista de Atención del Día**
- Tabla con las citas de hoy que siguen activas (excluye completadas,
  canceladas y no-asistidas), mostrando horario, paciente y motivo de
  consulta.
- Por cada cita pendiente, tres acciones:
  - **Iniciar Atención**: pide confirmación y navega a la pantalla de
    consulta (`/consulta/:id`).
  - **Reprogramar**: abre un modal con selector de fecha/hora
    (precargado con la fecha actual de la cita) para moverla a otro
    momento sin perder los demás datos de la cita.
  - **Cancelar**: pide confirmación (diálogo de confirmación con
    variante de advertencia) y marca la cita como cancelada.
- Las citas ya completadas se muestran con una marca "✓ Atendido" en vez
  de botones de acción.

---

## 5. Listado de Pacientes — `/pacientes`

- Tabla con todos los pacientes vinculados al doctor (nombre, datos de
  contacto).
- **Buscador** en tiempo real (typeahead) por nombre.
- **Paginación** real del listado general (controles Anterior / Página X
  de Y / Siguiente); la búsqueda no pagina, muestra resultados directos.
- Botón **"Ver Historial"** por paciente → navega al detalle
  (`/pacientes/:id`).
- Botón **"Eliminar"** por paciente: pide confirmación y desvincula al
  paciente del doctor actual (no borra al paciente del sistema — un mismo
  paciente puede estar vinculado a más de un doctor, así que la acción es
  "quitar de mi lista", no una destrucción de datos clínicos).
- Acceso para crear un paciente nuevo (`/pacientes/nuevo`).

## 6. Alta / Edición de Paciente — `/pacientes/nuevo`, `/pacientes/editar/:id`

Mismo formulario reutilizado para crear y editar.

- Campos: nombre, apellido, correo (opcional), teléfono, fecha de
  nacimiento, género (Masculino/Femenino/Otro).
- Validaciones básicas antes de enviar.
- Crea (`POST /patients`) o actualiza (`PUT /patients/:id`) según el modo.

## 7. Detalle de Paciente — `/pacientes/:id`

- Datos generales del paciente.
- Historial médico (antecedentes, notas clínicas guardadas).
- Línea de tiempo de citas del paciente (pasadas y futuras) con su
  estado.
- Botón **"Exportar Historial (PDF)"**: genera y descarga, directamente
  en el navegador (sin pasar por el backend), un PDF con el historial
  médico completo y la tabla de citas del paciente — útil para
  compartir o archivar fuera del sistema.

---

## 8. Calendario de Citas — `/calendar`

- Vista de calendario con todas las citas del doctor.
- **Filtro por estado**: Todas / Pendientes / Completadas / Canceladas /
  No asistió — recarga el calendario según el filtro elegido.
- Navegación entre meses/vistas del calendario, con botón "Hoy".
- Cada evento del calendario muestra paciente, horario y estado de la
  cita.

## 9. Nueva Cita — `/appointments/new`

- Dos modos de selección de paciente:
  - **Buscar paciente existente**: campo de búsqueda con autocompletado
    (espera ~3s tras dejar de escribir, mínimo 3 caracteres) contra el
    listado de pacientes.
  - **Registrar paciente nuevo** directamente desde el mismo formulario,
    sin salir a la pantalla de alta de pacientes.
- Puede abrirse pre-cargada con un paciente específico vía parámetro de
  URL (por ejemplo, desde el detalle de un paciente).
- Campos de la cita: fecha y hora (no permite fechas pasadas), motivo de
  consulta, observaciones opcionales.
- Al guardar, crea la cita (`POST /appointments`) asociada al paciente
  elegido/creado.

---

## 10. Consulta Médica — `/consulta/:appointmentId`

La pantalla más compleja del sistema: donde el doctor atiende al
paciente y documenta la consulta.

- Carga los datos de la cita y del paciente correspondiente.
- **Notas de consulta dinámicas**: secciones configurables por el propio
  doctor (ver pantalla de Ajustes de Notas), cada una con su campo de
  texto — permite adaptar la ficha clínica a la especialidad de cada
  doctor.
- **Formulario de datos del paciente** editable durante la consulta.
- **Carga de archivos/documentos** adjuntos a la consulta (estudios,
  imágenes, PDFs), con vista de los ya subidos.
- **Autoguardado de borrador**: mientras el doctor escribe, el progreso
  se guarda automáticamente para no perder información si se cierra la
  pestaña por accidente.
- **Restauración de borrador**: si había una consulta a medio terminar,
  al reabrirla se ofrece continuar desde el último borrador guardado.
- **Bloqueo de navegación con cambios sin guardar**: si el doctor intenta
  salir de la pantalla (cerrar pestaña, navegar hacia atrás) con cambios
  sin guardar, se le advierte antes de perderlos.
- **Generar Receta (PDF)**: genera una receta médica en PDF con los datos
  del doctor (incluyendo su leyenda/membrete configurado en su perfil) y
  del paciente, lista para descargar/imprimir.
- **Finalizar Consulta**: guarda todos los datos capturados (formulario
  del paciente, notas, archivos adjuntos pendientes de subir), marca la
  cita como completada y genera/guarda la receta correspondiente — deja
  la consulta cerrada.
- Si la cita ya fue completada previamente, la pantalla se bloquea en modo
  solo-lectura para evitar reeditar una consulta ya cerrada.

---

## 11. Perfil del Doctor — `/profile`

- Indicador de **porcentaje de perfil completado**.
- Datos verificados de solo lectura (no editables desde aquí, provienen
  de la validación de cédula): número de cédula, RFC, CURP.
- Datos editables: resumen/biografía profesional, nombre completo,
  especialidad médica, correo, teléfono, universidad, dirección, y la
  **leyenda de receta** (texto que aparece impreso en las recetas
  generadas en consulta).
- Carga de **foto de perfil** y **logo del consultorio** (imágenes,
  subidas como multipart al backend).
- Guarda todo junto (`PUT /doctor/me`).

## 12. Ajustes de Notas de Consulta — `/ajustes-notas`

Permite personalizar la ficha clínica que se usa en la pantalla de
Consulta Médica.

- Definir secciones/campos personalizados de notas: identificador,
  etiqueta visible, texto de ayuda (placeholder), si es obligatorio o no.
- Agregar y quitar secciones libremente.
- Guarda la plantilla en el backend (`POST /doctor/template`) y además la
  cachea en `localStorage` para carga instantánea la próxima vez.

---

## Navegación general — `DashboardLayout`

Estructura compartida por todas las pantallas protegidas (secciones 4–12).

- Barra lateral con accesos directos a: Dashboard (Panel de Control),
  Pacientes, Citas (Calendario), Perfil, Ajustes de Notas.
- **Cerrar sesión**: pide confirmación antes de salir y limpia el estado
  de sesión correctamente (evita que quede un aviso de "cambios sin
  guardar" fantasma de una consulta abierta).

---

## Resumen por función transversal (no ligada a una sola pantalla)

- **Autenticación y sesión**: JWT, login con usuario/contraseña o Google,
  onboarding en dos pasos obligatorio para cuentas nuevas.
- **Multiusuario por doctor**: cada doctor solo ve y gestiona sus propios
  pacientes y citas (aislamiento de datos entre cuentas).
- **Gestión de pacientes**: alta, edición, listado paginado, búsqueda,
  desvinculación ("eliminar"), historial clínico, exportación a PDF.
- **Gestión de citas**: agendar, calendario con filtros, cancelar,
  reprogramar, marcar como completada al finalizar la consulta.
- **Consulta médica**: notas configurables por especialidad, adjuntos,
  autoguardado, generación de recetas en PDF.
- **Métricas**: panel de estadísticas del consultorio (pacientes, citas
  del mes, inasistencias, próximas citas).
