import React from 'react';
import { Link } from 'react-router-dom';
import './LegalPage.scss';

// Cuerpo de los Términos y Condiciones, compartido entre la página pública
// standalone (TermsOfService.tsx) y la vista dentro del dashboard cuando
// hay sesión iniciada (App.tsx). Mismo patrón que PrivacyPolicyContent.
export const TermsOfServiceContent: React.FC = () => {
  return (
    <div className="legal-body">
      <Link to="/" className="legal-back-link">
        <span className="material-icons-outlined">arrow_back</span>
        Volver al inicio
      </Link>

      <div className="legal-draft-banner">
        <span className="material-icons-outlined">warning</span>
        <p>
          <strong>Este es un borrador.</strong> Todavía no ha sido revisado por un abogado y no debe
          considerarse válido legalmente. Antes de publicarlo de forma definitiva, un especialista
          debe revisarlo y completar los datos pendientes — sobre todo lo relativo a responsabilidad
          médica, cancelaciones y condiciones de la suscripción.
        </p>
      </div>

      <h1>Términos y Condiciones</h1>
      <p className="legal-updated">Última actualización: pendiente</p>

      <section>
        <h2>1. Aceptación de los términos</h2>
        <p>
          Al crear una cuenta o usar ProPatient (como doctor, personal de consultorio o paciente que
          agenda una cita), aceptas estos Términos y Condiciones. [Completar: nombre o razón social
          responsable del servicio, domicilio, datos de contacto.]
        </p>
      </section>

      <section>
        <h2>2. Descripción del servicio</h2>
        <p>
          ProPatient es una plataforma de gestión de consultorios médicos: agenda de citas,
          expediente clínico digital, recetas electrónicas y un directorio público donde los
          pacientes pueden encontrar doctores y solicitar citas sin crear una cuenta.
        </p>
      </section>

      <section>
        <h2>3. ProPatient no presta servicios médicos</h2>
        <p>
          ProPatient es una herramienta administrativa. <strong>No brinda diagnóstico, tratamiento
          ni consejo médico</strong>, y no es responsable por la atención médica que el doctor le
          brinde al paciente. La relación médico-paciente y la responsabilidad profesional son
          exclusivas del doctor. [Completar el alcance exacto de este deslinde con un abogado.]
        </p>
      </section>

      <section>
        <h2>4. Cuentas y responsabilidad del doctor</h2>
        <p>
          El doctor es responsable de la veracidad de su cédula profesional y demás datos
          proporcionados, de mantener segura su contraseña, y de las acciones que realice el
          personal de consultorio al que le dé acceso. [Completar el proceso de validación de
          cédula y las consecuencias de proporcionar información falsa.]
        </p>
      </section>

      <section>
        <h2>5. Suscripción y pagos</h2>
        <p>
          [Completar: duración de la prueba gratuita, precio de la suscripción, periodicidad de
          cobro, política de cancelación y reembolsos, qué pasa con los datos del consultorio si la
          suscripción vence o se cancela.]
        </p>
      </section>

      <section>
        <h2>6. Citas agendadas por pacientes</h2>
        <p>
          Un paciente puede solicitar una cita desde el directorio público sin crear una cuenta; esa
          solicitud queda sujeta a la confirmación del doctor. [Completar la política de
          cancelación/reprogramación y qué se le comunica al paciente por correo o WhatsApp.]
        </p>
      </section>

      <section>
        <h2>7. Protección de datos</h2>
        <p>
          El tratamiento de datos personales y de salud se rige por el{' '}
          <Link to="/privacidad">Aviso de Privacidad</Link>, que forma parte de estos Términos.
        </p>
      </section>

      <section>
        <h2>8. Limitación de responsabilidad</h2>
        <p>
          [Completar con un abogado: límites de responsabilidad de ProPatient por interrupciones del
          servicio, pérdida de datos, o fallas de los proveedores externos (correo, WhatsApp, pagos)
          que usa la plataforma.]
        </p>
      </section>

      <section>
        <h2>9. Cambios a estos términos</h2>
        <p>[Completar el procedimiento para notificar cambios a estos Términos y Condiciones.]</p>
      </section>

      <section>
        <h2>10. Contacto</h2>
        <p>[Completar: correo o medio de contacto para dudas sobre estos términos.]</p>
      </section>
    </div>
  );
};
