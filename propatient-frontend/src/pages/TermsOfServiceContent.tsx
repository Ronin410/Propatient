import React from 'react';
import { Link } from 'react-router-dom';
import './LegalPage.scss';

// Cuerpo de los Términos y Condiciones, compartido entre la página pública
// standalone (TermsOfService.tsx) y la vista dentro del dashboard cuando
// hay sesión iniciada (App.tsx). Mismo patrón que PrivacyPolicyContent.
//
// Combina el borrador original con el contenido útil de los seis
// documentos de referencia subidos en julio 2026 — en especial la
// estructura de PRO_Terminos_ProPatient Clinic.docx (la versión más completa de
// las dos que se subieron), rellenada con datos reales de cómo funciona
// la plataforma hoy (precio de prueba, plan de clínica, cancelación vía
// Stripe, etc.) en vez de dejarlo como texto genérico.
//
// A petición del usuario, sin el aviso de "borrador pendiente de abogado"
// visible en la página (fase de prueba con doctores reales antes de mandar
// el documento a revisión legal) — ver la conversación para el detalle de
// qué puntos siguen pendientes de validar con un especialista.
export const TermsOfServiceContent: React.FC = () => {
  return (
    <div className="legal-body">
      <Link to="/" className="legal-back-link">
        <span className="material-icons-outlined">arrow_back</span>
        Volver al inicio
      </Link>

      <h1>Términos y Condiciones</h1>
      <p className="legal-updated">Última actualización: 21 de julio de 2026</p>

      <section>
        <h2>1. Identidad del responsable y aceptación</h2>
        <p>
          ProPatient Clinic es una plataforma de software como servicio (SaaS) operada por Alejandro Bueno
          Mendoza en México. Al crear una cuenta como doctor, ProPatient Clinic te pide aceptar
          explícitamente estos Términos y el <Link to="/privacidad">Aviso de Privacidad</Link> antes
          de poder usar la plataforma; al agendar una cita como paciente desde el directorio público,
          se te pide aceptar el tratamiento de tus datos de salud conforme al Aviso de Privacidad.
        </p>
      </section>

      <section>
        <h2>2. Definiciones</h2>
        <ul>
          <li><strong>Usuario:</strong> el doctor o el personal de consultorio autorizado por él.</li>
          <li><strong>Paciente:</strong> la persona cuyos datos clínicos se gestionan en la plataforma.</li>
          <li><strong>Plataforma:</strong> el sitio web y la aplicación de ProPatient Clinic.</li>
        </ul>
      </section>

      <section>
        <h2>3. Descripción del servicio</h2>
        <p>
          ProPatient Clinic es una plataforma de gestión de consultorios médicos: agenda de citas,
          expediente clínico digital, recetas electrónicas, catálogo de diagnósticos CIE-10, y un
          directorio público donde los pacientes pueden encontrar doctores y solicitar citas sin
          crear una cuenta.
        </p>
      </section>

      <section>
        <h2>4. ProPatient Clinic no presta servicios médicos</h2>
        <p>
          ProPatient Clinic es una herramienta administrativa y de soporte documental. <strong>No brinda
          diagnóstico, tratamiento ni consejo médico</strong>, no evalúa la idoneidad de una receta ni
          interviene en ninguna decisión clínica. La relación médico-paciente y la responsabilidad
          profesional del acto médico son exclusivas del doctor, conforme a la NOM-004-SSA3-2012 y
          demás normatividad sanitaria aplicable.
        </p>
      </section>

      <section>
        <h2>5. Cuentas y responsabilidad del usuario</h2>
        <p>
          El doctor es responsable de la veracidad de su cédula profesional y demás datos
          proporcionados durante el registro, de mantener segura su contraseña y su sesión de Google,
          y de las acciones que realice cualquier persona de su personal (staff) a la que le dé acceso
          a la plataforma — incluida cualquier filtración o uso indebido derivado de credenciales que
          él mismo compartió o dejó expuestas.
        </p>
      </section>

      <section>
        <h2>6. Validación de la cédula profesional</h2>
        <p>
          Antes de poder usar la plataforma con normalidad, todo doctor debe capturar su cédula
          profesional y una identificación oficial, que un administrador revisa manualmente. Si la
          información proporcionada resulta falsa o no puede verificarse, ProPatient Clinic puede rechazar o
          suspender la cuenta sin responsabilidad alguna de su parte.
        </p>
      </section>

      <section>
        <h2>7. Suscripción y pagos</h2>
        <p>Todo doctor nuevo cuenta con 14 días de prueba gratuita. Al concluir ese periodo:</p>
        <ul>
          <li>
            <strong>Plan individual:</strong> $1,200.00 MXN mensuales, cobrados automáticamente de
            forma recurrente a través de Stripe (pasarela de pago externa; ProPatient Clinic nunca ve ni
            almacena el número de tarjeta). Puedes cancelar en cualquier momento desde el Portal de
            Cliente de Stripe — la cancelación detiene los cobros futuros, pero no genera reembolso
            de periodos ya cobrados.
          </li>
          <li>
            <strong>Plan de clínica:</strong> tarifa base de $3,200.00 MXN mensuales (incluye hasta 5
            doctores vinculados), más $1,000.00 MXN mensuales por cada doctor adicional. Este plan no
            tiene periodo de prueba.
          </li>
          <li>
            Si tu suscripción vence o se cancela, tu cuenta queda inhabilitada para iniciar sesión,
            pero el expediente clínico de tus pacientes se conserva conforme a la sección 13 — no se
            pierde ni se borra de forma automática.
          </li>
          <li>
            Si consideras que se vulneró alguno de tus derechos como consumidor en el cobro de tu
            suscripción, puedes presentar tu queja ante la Procuraduría Federal del Consumidor
            (PROFECO) a través de{' '}
            <a href="https://www.profeco.gob.mx" target="_blank" rel="noopener noreferrer">
              www.profeco.gob.mx
            </a>, independientemente de cualquier otro medio de contacto directo con ProPatient Clinic.
          </li>
        </ul>
      </section>

      <section>
        <h2>8. Citas agendadas por pacientes</h2>
        <p>
          Un paciente (o su padre, madre o tutor, si es menor de edad) puede solicitar una cita desde
          el directorio público sin crear una cuenta. Esa solicitud queda pendiente hasta que el
          doctor o su personal la confirme o la rechace — no se considera una cita agendada en firme
          hasta ese momento. La cancelación o reprogramación de una cita ya confirmada se maneja
          directamente entre el consultorio y el paciente.
        </p>
      </section>

      <section>
        <h2>9. Disponibilidad del servicio</h2>
        <p>
          ProPatient Clinic se ofrece "tal como está" ("as is"), sin garantía de disponibilidad
          ininterrumpida. En particular, ProPatient Clinic no es responsable por:
        </p>
        <ul>
          <li>
            La falta de entrega de un recordatorio o aviso por fallas de un proveedor externo
            (WhatsApp/Twilio, correo/Resend).
          </li>
          <li>
            La pérdida de sincronización con Google Calendar si el propio doctor desconectó la
            integración o Google interrumpe su servicio.
          </li>
          <li>
            Caídas generales de la infraestructura en la nube provista por el proveedor de hosting.
          </li>
        </ul>
      </section>

      <section>
        <h2>10. Limitación de responsabilidad</h2>
        <p>
          ProPatient Clinic no es responsable por decisiones médicas, diagnósticos, tratamientos ni por
          cualquier consecuencia derivada del ejercicio profesional del doctor. Salvo negligencia
          grave comprobada de la plataforma, la responsabilidad económica total de ProPatient Clinic frente
          al doctor, por cualquier concepto, queda limitada al importe efectivamente pagado por su
          suscripción en los tres meses previos al reclamo.
        </p>
      </section>

      <section>
        <h2>11. Propiedad intelectual</h2>
        <p>
          El software, el diseño y la marca ProPatient Clinic son propiedad de su operador. El doctor
          conserva la titularidad de la información clínica que él mismo genera sobre sus pacientes;
          ProPatient Clinic solo la trata como encargado, conforme al <Link to="/privacidad">Aviso de
          Privacidad</Link>.
        </p>
      </section>

      <section>
        <h2>12. Terminación</h2>
        <p>
          ProPatient Clinic puede suspender o cancelar una cuenta que incumpla estos Términos, use la
          plataforma para fines ilícitos, o proporcione información falsa durante el registro o la
          validación de cédula. El doctor puede cancelar su propia cuenta en cualquier momento desde
          su Perfil, siempre que no tenga una suscripción activa sin cancelar primero.
        </p>
      </section>

      <section>
        <h2>13. Retención de datos tras la cancelación</h2>
        <p>
          Cancelar la cuenta desactiva el acceso de inmediato, pero no borra en cascada los datos de
          los pacientes ni el expediente clínico — se conservan por el plazo mínimo que exige la
          normatividad sanitaria (ver <Link to="/privacidad">Aviso de Privacidad</Link>, sección de
          conservación de datos). La eliminación física definitiva solo procede a solicitud expresa y
          por escrito, una vez cumplido ese plazo.
        </p>
      </section>

      <section>
        <h2>14. Modificaciones a estos términos</h2>
        <p>
          Estos Términos pueden actualizarse por cambios en la plataforma o en la legislación
          aplicable. Cualquier cambio relevante se publicará en esta misma página con su fecha de
          actualización; si el cambio es sustancial, se te pedirá aceptarlo de nuevo antes de seguir
          usando la plataforma.
        </p>
      </section>

      <section>
        <h2>15. Ley aplicable y contacto</h2>
        <p>
          Estos Términos se rigen por las leyes federales de los Estados Unidos Mexicanos, y cualquier
          controversia derivada de ellos se someterá a los tribunales competentes en territorio
          mexicano, renunciando las partes a cualquier otro fuero que pudiera corresponderles. Para
          dudas sobre estos Términos, escribe a{' '}
          <a href="mailto:alejandrobuenomendoza@gmail.com">alejandrobuenomendoza@gmail.com</a>.
        </p>
      </section>
    </div>
  );
};
