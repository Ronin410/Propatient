import React from 'react';
import { Link } from 'react-router-dom';
import { Footer } from '../components/Footer';
import { PrivacyPolicyContent } from './PrivacyPolicyContent';
import logo from '../assets/logo.png';
import './LegalPage.scss';

// Versión pública standalone (sin sesión iniciada): trae su propia
// cabecera de navegación y footer. Con sesión iniciada, App.tsx muestra
// PrivacyPolicyContent dentro del DashboardLayout en su lugar, para
// conservar el menú lateral.
export const PrivacyPolicy: React.FC = () => {
  return (
    <div className="legal-page">
      <header className="legal-nav">
        <Link to="/" className="legal-logo">
          <img src={logo} alt="ProPatient Clinic" className="brand-logo-icon" />
          ProPatient Clinic
        </Link>
      </header>

      <PrivacyPolicyContent />

      <Footer />
    </div>
  );
};
