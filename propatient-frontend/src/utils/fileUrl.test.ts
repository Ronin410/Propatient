import { describe, expect, it, vi, beforeEach } from 'vitest';

describe('toAbsoluteFileUrl', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it('deja pasar sin cambios una URL ya absoluta (S3 firmada)', async () => {
    const { toAbsoluteFileUrl } = await import('./fileUrl');
    const signed = 'https://mi-bucket.s3.amazonaws.com/recipes/receta_1_123.pdf?X-Amz-Signature=abc';
    expect(toAbsoluteFileUrl(signed)).toBe(signed);
  });

  it('antepone el origen del backend a un path relativo (disco local)', async () => {
    const { toAbsoluteFileUrl } = await import('./fileUrl');
    expect(toAbsoluteFileUrl('/uploads/recipes/receta_1_123.pdf')).toContain('/uploads/recipes/receta_1_123.pdf');
  });

  it('agrega el "/" inicial si falta', async () => {
    const { toAbsoluteFileUrl } = await import('./fileUrl');
    const result = toAbsoluteFileUrl('uploads/documents/1_123.pdf');
    expect(result.endsWith('/uploads/documents/1_123.pdf')).toBe(true);
  });

  it('devuelve cadena vacía para valores vacíos/nulos', async () => {
    const { toAbsoluteFileUrl } = await import('./fileUrl');
    expect(toAbsoluteFileUrl('')).toBe('');
    expect(toAbsoluteFileUrl(undefined)).toBe('');
    expect(toAbsoluteFileUrl(null)).toBe('');
  });
});
