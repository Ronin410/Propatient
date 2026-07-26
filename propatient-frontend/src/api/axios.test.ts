import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Regresión del bug real de producción: VITE_API_URL="propatient" (sin
// esquema) hacía que axios armara una URL relativa y le pegara al propio
// frontend estático en vez del backend. Cada test recarga el módulo con un
// import.meta.env distinto para simular diferentes valores de build.

describe('api/axios BACKEND_ORIGIN y baseURL', () => {
  const originalConsoleError = console.error;

  beforeEach(() => {
    vi.resetModules();
    console.error = vi.fn();
  });

  afterEach(() => {
    console.error = originalConsoleError;
    vi.unstubAllEnvs();
  });

  it('usa el fallback local si VITE_API_URL no está definida', async () => {
    vi.stubEnv('VITE_API_URL', '');
    const { default: api, BACKEND_ORIGIN } = await import('./axios');
    expect(api.defaults.baseURL).toBe('http://localhost:8095/api');
    expect(BACKEND_ORIGIN).toBe('http://localhost:8095');
  });

  it('usa VITE_API_URL cuando es una URL absoluta válida', async () => {
    vi.stubEnv('VITE_API_URL', 'https://propatient-api.onrender.com/api');
    const { default: api, BACKEND_ORIGIN } = await import('./axios');
    expect(api.defaults.baseURL).toBe('https://propatient-api.onrender.com/api');
    expect(BACKEND_ORIGIN).toBe('https://propatient-api.onrender.com');
  });

  it('cae al fallback y avisa en consola si VITE_API_URL no es una URL absoluta', async () => {
    vi.stubEnv('VITE_API_URL', 'propatient');
    const { default: api } = await import('./axios');
    expect(api.defaults.baseURL).toBe('http://localhost:8095/api');
    expect(console.error).toHaveBeenCalledWith(expect.stringContaining('VITE_API_URL="propatient"'));
  });
});
