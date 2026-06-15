import axios from 'axios';

const api = axios.create({
  // IMPORTANTE: No pongas una barra "/" al final de la URL base
  baseURL: 'http://localhost:8095/api', 
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