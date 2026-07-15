import React from 'react';
import { Link } from 'react-router-dom';
import { Footer } from '../components/Footer';
import './PrivacyPolicy.scss';

export const PrivacyPolicy: React.FC = () => {
  return (
    <div className="privacy-page">
      <header className="privacy-nav">
        <Link to="/" className="privacy-logo">
          <span className="material-icons-outlined">favorite</span>
          ProPatient
        </Link>
      </header>

      <div className="privacy-body">
        <div className="privacy-draft-banner">
          <span className="material-icons-outlined">warning</span>
          <p>
            <strong>Este es un borrador.</strong> Todavía no ha sido revisado por un abogado y no debe
            considerarse válido legalmente. Al tratarse de datos de salud (datos personales sensibles
            bajo la LFPDPPP), es indispensable que un especialista en protección de datos lo revise y
            complete antes de publicarlo de forma definitiva.
          </p>
        </div>

        <h1>Aviso de Privacidad</h1>
        <p className="privacy-updated">Última actualización: pendiente</p>

        <section>
          <h2>1. Responsable del tratamiento de datos</h2>
          <p>[Completar: nombre o razón social del responsable, domicilio, datos de contacto.]</p>
        </section>

        <section>
          <h2>2. Datos personales que se recaban</h2>
          <p>
            A través de ProPatient se recaban datos de identificación y contacto de doctores, personal
            de consultorio y pacientes (nombre, correo, teléfono, dirección), así como datos clínicos
            del expediente médico (antecedentes, diagnósticos, tratamientos, recetas).
          </p>
        </section>

        <section>
          <h2>3. Datos sensibles</h2>
          <p>
            Los datos de salud que se registran en el expediente clínico son <strong>datos personales
            sensibles</strong> conforme a la Ley Federal de Protección de Datos Personales en Posesión
            de los Particulares (LFPDPPP). Su tratamiento requiere consentimiento expreso y por escrito
            del titular. [Completar el mecanismo de consentimiento utilizado.]
          </p>
        </section>

        <section>
          <h2>4. Finalidades del tratamiento</h2>
          <p>
            [Completar: agendar y dar seguimiento a citas médicas, integrar el expediente clínico,
            generar recetas electrónicas, enviar recordatorios y confirmaciones por correo/WhatsApp,
            facturación de la suscripción, y demás finalidades necesarias para operar el consultorio.]
          </p>
        </section>

        <section>
          <h2>5. Derechos ARCO</h2>
          <p>
            El titular de los datos tiene derecho a Acceder, Rectificar, Cancelar u Oponerse (derechos
            ARCO) al tratamiento de sus datos personales. [Completar el procedimiento y medio para
            ejercer estos derechos.]
          </p>
        </section>

        <section>
          <h2>6. Transferencia de datos</h2>
          <p>[Completar: si se comparten datos con terceros — proveedores de correo, WhatsApp, pagos — y bajo qué condiciones.]</p>
        </section>

        <section>
          <h2>7. Cambios a este aviso</h2>
          <p>[Completar el procedimiento para notificar cambios a este aviso de privacidad.]</p>
        </section>

        <section>
          <h2>8. Contacto</h2>
          <p>[Completar: correo o medio de contacto para dudas sobre privacidad.]</p>
        </section>
      </div>

      <Footer />
    </div>
  );
};
