import axios from 'axios';

// Extrae el mensaje de error que manda el backend (gin.H{"error": "..."})
// de un catch (err: unknown), con un fallback si no viene o no es un error de axios.
export function getErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError(err)) {
    return err.response?.data?.error || fallback;
  }
  return fallback;
}
