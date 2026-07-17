import { describe, expect, it } from 'vitest';
import { sanitizePhoneInput } from './phoneInput';

describe('sanitizePhoneInput', () => {
  it('deja pasar solo dígitos', () => {
    expect(sanitizePhoneInput('5512345678')).toBe('5512345678');
  });

  it('quita letras y símbolos', () => {
    expect(sanitizePhoneInput('abc55-12/34*5678xyz')).toBe('5512345678');
  });

  it('conserva un "+" inicial (código de país explícito)', () => {
    expect(sanitizePhoneInput('+52 55 1234 5678')).toBe('+525512345678');
  });

  it('quita un "+" que no esté al inicio', () => {
    expect(sanitizePhoneInput('55+1234+5678')).toBe('5512345678');
  });

  it('devuelve cadena vacía si no queda ningún dígito', () => {
    expect(sanitizePhoneInput('abc')).toBe('');
    expect(sanitizePhoneInput('+abc')).toBe('+');
  });
});
