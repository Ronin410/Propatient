import React from 'react';
import { Link } from 'react-router-dom';
import { Footer } from '../components/Footer';
import { TermsOfServiceContent } from './TermsOfServiceContent';
import logo from '../assets/logo.png';
import './LegalPage.scss';

// Versión pública standalone (sin sesión iniciada): trae su propia
// cabecera de navegación y footer. Con sesión iniciada, App.tsx muestra
// TermsOfServiceContent dentro del DashboardLayout en su lugar, para
// conservar el menú lateral. Mismo patrón que PrivacyPolicy.tsx.
export const TermsOfService: React.FC = () => {
  return (
    <div className="legal-page">
      <header className="legal-nav">
        <Link to="/" className="legal-logo">
          <img src={logo} alt="ProPatient" className="brand-logo-icon" />
          ProPatient
        </Link>
      </header>

      <TermsOfServiceContent />

      <Footer />
    </div>
  );
};
