import axios from 'axios';

const LOCAL_API_URL = 'http://localhost:8095/api';

// Base del API (incluye "/api"). Se define en build time vía VITE_API_URL
// (ver .env.example); si no está presente, cae al backend local de desarrollo.
// Si está definida pero no es una URL absoluta (falta "http(s)://", ej. por
// pegar solo el nombre del servicio de Render por error), axios la trataría
// como ruta relativa y terminaría pegándole al propio frontend en vez del
// backend. Mejor fallar de forma ruidosa en consola que fallar en silencio.
const rawApiUrl = import.meta.env.VITE_API_URL as string | undefined;
const API_BASE_URL = (() => {
  if (!rawApiUrl) return LOCAL_API_URL;
  if (/^https?:\/\//.test(rawApiUrl)) return rawApiUrl;
  console.error(
    `VITE_API_URL="${rawApiUrl}" no es una URL absoluta (debe empezar con ` +
    `http:// o https://). Usando "${LOCAL_API_URL}" como fallback; revisa ` +
    `la variable de entorno en el build del frontend.`
  );
  return LOCAL_API_URL;
})();

// Origen del backend sin el sufijo "/api", para armar URLs de archivos
// estáticos (avatares, logos, documentos) que el backend expone en /uploads.
export const BACKEND_ORIGIN = API_BASE_URL.replace(/\/api\/?$/, '');

const api = axios.create({
  // IMPORTANTE: No pongas una barra "/" al final de la URL base
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Interceptor de Peticiones: Añade el Token automáticamente
api.interceptors.request.use(
  (config) => {
    // 1. Verificamos el token en el Storage
    const token = localStorage.getItem('auth_token');
    
    // 2. Si el token existe, lo inyectamos
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }

    // 3. LIMPIEZA DE URL (Anti-errores de Go/Gin)
    // Esto elimina barras diagonales dobles o finales que causan el error 301/404
    if (config.url && config.url.endsWith('/')) {
        config.url = config.url.slice(0, -1);
    }

    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Interceptor de Respuestas: Manejo de seguridad global
api.interceptors.response.use(
  (response) => response,
  (error) => {
    // Si el backend devuelve 401 (Unauthorized), el token no sirve
    if (error.response?.status === 401) {
      console.warn("Sesión no autorizada o expirada.");
      
      // Solo limpiamos y redirigimos si no estamos intentando loguearnos
      if (!window.location.pathname.includes('/login')) {
        localStorage.removeItem('auth_token');
        localStorage.removeItem('user_data');
        window.location.href = '/login';
      }
    }

    // Manejo de errores de red o servidor apagado
    if (!error.response) {
      console.error("Error de red: No se pudo conectar con el servidor Go en el puerto 8095");
    }
    
    return Promise.reject(error);
  }
);

export default api;