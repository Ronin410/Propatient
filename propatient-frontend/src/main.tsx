import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import * as Sentry from '@sentry/react'
import App from './App.tsx' // Importa el componente App
import { ThemeProvider } from './context/ThemeContext'
import './index.css'

// Sin VITE_SENTRY_DSN configurada, Sentry.init nunca se llama y el resto
// de la app funciona exactamente igual — mismo patrón de integración
// opcional que el resto del proyecto (reCAPTCHA, S3, WhatsApp).
const sentryDsn = import.meta.env.VITE_SENTRY_DSN as string | undefined;
if (sentryDsn) {
  Sentry.init({
    dsn: sentryDsn,
    environment: (import.meta.env.VITE_SENTRY_ENVIRONMENT as string | undefined) || 'production',
    tracesSampleRate: 0.2,
  });
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Sentry.ErrorBoundary fallback={<ErrorFallback />}>
      <ThemeProvider>
        <App />
      </ThemeProvider>
    </Sentry.ErrorBoundary>
  </StrictMode>,
)

// Pantalla mínima si React se cae por completo (no debería pasar en uso
// normal) — mejor que una página en blanco sin ninguna pista de qué hacer.
function ErrorFallback() {
  return (
    <div style={{ padding: '3rem', textAlign: 'center', fontFamily: 'sans-serif' }}>
      <h1>Algo salió mal</h1>
      <p>Recarga la página. Si el problema sigue, contáctanos.</p>
    </div>
  );
}