import { BACKEND_ORIGIN } from '../api/axios';

// El backend devuelve rutas de archivo de dos formas posibles: un path
// relativo servido por el propio backend ("/uploads/...", cuando no hay S3
// configurado) o una URL ya absoluta y firmada de S3 ("https://...", cuando
// sí lo hay). Este helper las normaliza a algo siempre usable directamente
// en <img src>, <a href>, etc., sin que cada pantalla tenga que repetir la
// misma comprobación.
export function toAbsoluteFileUrl(path: string | undefined | null): string {
  if (!path) return '';
  if (/^https?:\/\//i.test(path)) return path;
  return `${BACKEND_ORIGIN}${path.startsWith('/') ? path : `/${path}`}`;
}
