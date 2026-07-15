import React from 'react';
import { Link } from 'react-router-dom';
import { Footer } from '../components/Footer';
import { PrivacyPolicyContent } from './PrivacyPolicyContent';
import './PrivacyPolicy.scss';

// Versión pública standalone (sin sesión iniciada): trae su propia
// cabecera de navegación y footer. Con sesión iniciada, App.tsx muestra
// PrivacyPolicyContent dentro del DashboardLayout en su lugar, para
// conservar el menú lateral.
export const PrivacyPolicy: React.FC = () => {
  return (
    <div className="privacy-page">
      <header className="privacy-nav">
        <Link to="/" className="privacy-logo">
          <span className="material-icons-outlined">favorite</span>
          ProPatient
        </Link>
      </header>

      <PrivacyPolicyContent />

      <Footer />
    </div>
  );
};
