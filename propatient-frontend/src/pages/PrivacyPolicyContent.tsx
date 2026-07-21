import React from 'react';
import { Link } from 'react-router-dom';
import './LegalPage.scss';

// Cuerpo del aviso de privacidad, compartido entre la página pública
// standalone (PrivacyPolicy.tsx) y la vista dentro del dashboard cuando
// hay sesión iniciada (App.tsx), para no duplicar el contenido.
//
// Combina el borrador original con el contenido útil de los seis
// documentos de referencia subidos en julio 2026 (Seguridad, Cookies,
// Consentimiento, Privacidad, Términos x2) — corrigiendo lo que no
// aplicaba a como funciona ProPatient de verdad (ej. el borrador de
// Cookies decía que se usan cookies para autenticación; la sesión en
// realidad se guarda como JWT en localStorage, no en una cookie propia).
//
// A petición del usuario, sin el aviso de "borrador pendiente de abogado"
// visible en la página (fase de prueba con doctores reales antes de mandar
// el documento a revisión legal) — ver la conversación para el detalle de
// qué puntos siguen pendientes de validar con un especialista.
export const PrivacyPolicyContent: React.FC = () => {
  return (
    <div className="legal-body">
      <Link to="/" className="legal-back-link">
        <span className="material-icons-outlined">arrow_back</span>
        Volver al inicio
      </Link>

      <h1>Aviso de Privacidad</h1>
      <p className="legal-updated">Última actualización: 21 de julio de 2026</p>

      <section>
        <h2>1. Responsable del tratamiento de datos</h2>
        <p>
          El responsable del tratamiento de los datos personales recabados a través de ProPatient es
          Alejandro Bueno Mendoza, operando actualmente como persona física con actividad empresarial
          conforme a las leyes de México. El domicilio del responsable, para efectos de este aviso,
          está disponible previa solicitud a través del siguiente correo de contacto. Para cualquier
          duda o solicitud relacionada con este aviso, puedes escribir a{' '}
          <a href="mailto:alejandrobuenomendoza@gmail.com">alejandrobuenomendoza@gmail.com</a>.
        </p>
      </section>

      <section>
        <h2>2. Datos personales que se recaban</h2>
        <p>ProPatient recaba datos distintos según quién los proporciona:</p>
        <ul>
          <li>
            <strong>Doctores:</strong> nombre completo, correo, teléfono, domicilio del consultorio,
            fecha de nacimiento, cédula profesional, universidad de egreso, y copia de identificación
            oficial para validar la cédula. Si activa el cobro de suscripción, también un identificador
            de referencia de Stripe (ProPatient nunca almacena el número de tarjeta).
          </li>
          <li>
            <strong>Personal de consultorio (staff):</strong> nombre completo y correo electrónico; su
            contraseña se guarda siempre cifrada (hash), nunca en texto plano.
          </li>
          <li>
            <strong>Pacientes:</strong> nombre, fecha de nacimiento, género, teléfono y correo de
            contacto — o, si el paciente es menor de edad, los de quien ejerce su patria potestad o
            tutela — además de los datos clínicos descritos en la sección siguiente.
          </li>
        </ul>
      </section>

      <section>
        <h2>3. Datos personales sensibles</h2>
        <p>
          Los datos de salud que se registran en el expediente clínico (alergias, antecedentes
          patológicos y no patológicos, historial gineco-obstétrico, medicación actual, diagnósticos —
          incluido el código CIE-10 cuando el doctor lo usa —, notas de evolución y recetas) son{' '}
          <strong>datos personales sensibles</strong> conforme a la legislación mexicana de protección
          de datos. Su tratamiento requiere el consentimiento expreso del titular (o de su padre,
          madre o tutor legal si es menor de edad) antes de agendar una cita.
        </p>
      </section>

      <section>
        <h2>4. Finalidades del tratamiento</h2>
        <p>Los datos se usan para:</p>
        <ul>
          <li>Agendar y dar seguimiento a citas médicas.</li>
          <li>Integrar y conservar el expediente clínico digital del paciente.</li>
          <li>Generar recetas electrónicas y documentos clínicos.</li>
          <li>Validar la identidad y la cédula profesional del doctor.</li>
          <li>
            Enviar confirmaciones, recordatorios y avisos por correo electrónico, WhatsApp o
            notificación push (esta última solo si el doctor la activa desde su Perfil).
          </li>
          <li>Procesar el cobro recurrente de la suscripción, cuando aplica.</li>
          <li>Mostrar el perfil del doctor en el directorio público, solo si él decide activarlo.</li>
        </ul>
      </section>

      <section>
        <h2>5. Consentimiento</h2>
        <p>
          Para datos sensibles de salud, ProPatient pide consentimiento expreso y guarda evidencia de
          esa aceptación — no basta con que el formulario lo exija visualmente. Al doctor se le pide
          aceptar los Términos y este Aviso de Privacidad la primera vez que entra a la plataforma; al
          paciente (o a su tutor, si es menor de edad) se le pide aceptar el tratamiento de sus datos
          de salud al agendar una cita desde el directorio público. En ambos casos se registra
          automáticamente la fecha, la dirección IP y la versión del aviso vigente al momento de
          aceptar, como constancia de que el consentimiento se dio de forma real y verificable.
        </p>
      </section>

      <section>
        <h2>6. Medidas de seguridad</h2>
        <p>Dada la sensibilidad de los datos de salud almacenados, ProPatient implementa:</p>
        <ul>
          <li>
            <strong>Cifrado en tránsito:</strong> toda la comunicación entre el navegador y los
            servidores viaja por HTTPS/TLS.
          </li>
          <li>
            <strong>Cifrado en reposo:</strong> la base de datos y los archivos almacenados (AES-256)
            están cifrados en la infraestructura del proveedor de hosting.
          </li>
          <li>
            <strong>Contraseñas cifradas:</strong> ninguna contraseña se guarda en texto plano —se
            almacena únicamente su hash.
          </li>
          <li>
            <strong>Control de acceso por rol:</strong> el personal de consultorio (staff) tiene acceso
            limitado y nunca puede ver el expediente clínico completo ni la información de facturación
            del doctor.
          </li>
          <li>
            <strong>Bitácora de auditoría inmutable:</strong> cada acceso o modificación a un
            expediente clínico queda registrado (quién, qué, cuándo, desde qué dirección IP), y las
            versiones anteriores de una nota clínica se conservan aunque el doctor la edite después —
            nunca se sobreescriben ni se borran.
          </li>
        </ul>
      </section>

      <section>
        <h2>7. Uso de cookies</h2>
        <p>
          ProPatient <strong>no utiliza cookies propias para iniciar tu sesión</strong>: al entrar,
          la plataforma guarda un token de acceso cifrado directamente en tu navegador (almacenamiento
          local), no en una cookie. Las únicas cookies que pueden aparecer al usar el sitio son las que
          coloca el propio botón de inicio de sesión de Google (necesarias para que funcione ese
          inicio de sesión) — ProPatient no las controla ni las usa con fines publicitarios, y puedes
          administrarlas desde la configuración de cookies de tu navegador o de tu cuenta de Google.
        </p>
      </section>

      <section>
        <h2>8. Transferencia de datos a proveedores tecnológicos</h2>
        <p>
          ProPatient no vende ni comercializa tus datos. Para poder operar, algunos datos se comparten
          con proveedores tecnológicos que actúan como encargados del tratamiento, únicamente para la
          finalidad indicada:
        </p>
        <ul>
          <li><strong>Render:</strong> aloja el servidor y la base de datos de la plataforma.</li>
          <li>
            <strong>Amazon Web Services (S3):</strong> almacena de forma privada los documentos
            adjuntos (identificaciones, resultados, recetas en PDF), cuando está configurado.
          </li>
          <li><strong>Stripe:</strong> procesa los pagos de la suscripción.</li>
          <li>
            <strong>Twilio:</strong> envía los avisos y recordatorios por WhatsApp, cuando está
            configurado.
          </li>
          <li><strong>Resend:</strong> envía los correos automáticos de la plataforma.</li>
          <li>
            <strong>Sentry:</strong> recibe únicamente información técnica de errores del sistema
            (nunca el contenido de un expediente clínico) para poder corregir fallas.
          </li>
        </ul>
      </section>

      <section>
        <h2>9. Uso de la API de Google Calendar</h2>
        <p>
          Si el doctor elige conectar su cuenta de Google Calendar desde su Perfil (función opcional,
          desactivada por defecto), ProPatient solicita permiso de Google mediante el alcance
          (scope) <code>https://www.googleapis.com/auth/calendar.events</code> — acceso limitado a la
          creación, edición y eliminación de eventos de calendario, sin acceso a ningún otro dato de
          la cuenta de Google del doctor (correo, contactos, archivos, etc.).
        </p>
        <p>
          Este permiso se usa exclusivamente para crear un evento "espejo" en el Google Calendar del
          doctor por cada cita registrada en ProPatient, y mantenerlo sincronizado (actualizarlo si la
          cita se reprograma, eliminarlo si se cancela). ProPatient no lee, exporta ni almacena el
          contenido de otros eventos que ya existieran en el calendario del doctor antes de conectar
          la cuenta — únicamente gestiona los eventos que él mismo crea a partir de esta integración.
        </p>
        <p>
          El doctor puede revocar este acceso en cualquier momento desde su Perfil ("Desconectar
          Google Calendar") o directamente desde la configuración de su cuenta de Google
          (<a href="https://myaccount.google.com/permissions" target="_blank" rel="noopener noreferrer">myaccount.google.com/permissions</a>).
          Al desconectarlo, ProPatient deja de crear o modificar eventos, y no conserva ningún dato
          proveniente de Google Calendar más allá de lo estrictamente necesario mientras la conexión
          estuvo activa.
        </p>
        <p>
          El uso y la transferencia a cualquier otra aplicación de la información recibida de las API
          de Google se adhieren a la
          {' '}<a href="https://developers.google.com/terms/api-services-user-data-policy" target="_blank" rel="noopener noreferrer">
            Política de Datos de Usuario de los Servicios de API de Google
          </a>, incluidos los requisitos de Uso Limitado (Limited Use).
        </p>
      </section>

      <section>
        <h2>10. Conservación de datos</h2>
        <p>
          El expediente clínico se conserva por un periodo mínimo de 5 años a partir del último acto
          médico registrado, conforme a la normatividad sanitaria mexicana sobre expedientes clínicos.
          Si un doctor cancela su cuenta, ProPatient desactiva el acceso de inmediato pero no borra el
          expediente clínico de sus pacientes durante ese plazo — solo se elimina de forma definitiva a
          solicitud expresa y por escrito, una vez que ya no exista una obligación legal de
          conservarlo.
        </p>
      </section>

      <section>
        <h2>11. Derechos ARCO</h2>
        <p>
          Puedes Acceder, Rectificar, Cancelar u Oponerte (derechos ARCO) al tratamiento de tus datos
          personales en cualquier momento. El doctor puede ejercerlos directamente desde su Perfil
          (exportar o eliminar su cuenta y datos asociados). Cualquier otro titular (personal o
          paciente) puede solicitarlo escribiendo a{' '}
          <a href="mailto:alejandrobuenomendoza@gmail.com">alejandrobuenomendoza@gmail.com</a>,
          indicando claramente su nombre, el derecho que desea ejercer y una forma de verificar su
          identidad. Responderemos a la brevedad posible, respetando los plazos que marque la
          legislación aplicable vigente al momento de la solicitud.
        </p>
        <p>
          Los datos clínicos de un paciente no pueden eliminarse antes de que se cumpla el periodo
          mínimo de conservación legal (sección 10), incluso si el paciente ejerce su derecho de
          cancelación u oposición — se trata de una excepción prevista por la propia normatividad
          sanitaria.
        </p>
      </section>

      <section>
        <h2>12. Cambios a este aviso</h2>
        <p>
          Este aviso puede actualizarse por cambios en la plataforma o en la legislación aplicable.
          Cualquier cambio relevante se publicará en esta misma página con su fecha de actualización.
          Si el cambio afecta de forma sustancial cómo tratamos tus datos, se te pedirá aceptar la
          nueva versión antes de seguir usando la plataforma.
        </p>
      </section>
    </div>
  );
};
