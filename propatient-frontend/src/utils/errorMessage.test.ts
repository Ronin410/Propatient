import { describe, expect, it } from 'vitest';
import { AxiosError, AxiosHeaders } from 'axios';
import { getErrorMessage } from './errorMessage';

function makeAxiosError(data: unknown, status = 400): AxiosError {
  return new AxiosError(
    'Request failed',
    'ERR_BAD_REQUEST',
    undefined,
    undefined,
    {
      status,
      statusText: 'Bad Request',
      headers: {},
      config: { headers: new AxiosHeaders() },
      data,
    }
  );
}

describe('getErrorMessage', () => {
  it('devuelve el mensaje de error que manda el backend', () => {
    const err = makeAxiosError({ error: 'Usuario o contraseña incorrectos' });
    expect(getErrorMessage(err, 'fallback')).toBe('Usuario o contraseña incorrectos');
  });

  it('usa el fallback si la respuesta de axios no trae "error"', () => {
    const err = makeAxiosError({});
    expect(getErrorMessage(err, 'fallback')).toBe('fallback');
  });

  it('usa el fallback si no es un AxiosError (ej. network error puro)', () => {
    expect(getErrorMessage(new Error('network fail'), 'fallback')).toBe('fallback');
  });

  it('usa el fallback si el valor lanzado no es ni siquiera un Error', () => {
    expect(getErrorMessage('algo raro', 'fallback')).toBe('fallback');
    expect(getErrorMessage(undefined, 'fallback')).toBe('fallback');
  });
});
