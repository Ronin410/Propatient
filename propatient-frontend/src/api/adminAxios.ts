import axios from 'axios';
import { API_BASE_URL } from './axios';

// Cliente aparte para el panel interno de administración (/admin): usa su
// propia llave de sesión ("admin_token", NUNCA "auth_token") para no
// mezclarse con la sesión del doctor que pueda estar abierta en el mismo
// navegador — reusar el cliente "api" normal inyectaría el token del
// doctor en las peticiones de admin, y su interceptor de 401 borraría la
// sesión del doctor y redirigiría a /login, que no es lo que queremos aquí.
const adminApi = axios.create({
  baseURL: API_BASE_URL,
  headers: { 'Content-Type': 'application/json' },
});

adminApi.interceptors.request.use((config) => {
  const token = localStorage.getItem('admin_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

adminApi.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && !window.location.pathname.includes('/admin/login')) {
      localStorage.removeItem('admin_token');
      window.location.href = '/admin/login';
    }
    return Promise.reject(error);
  }
);

export default adminApi;
