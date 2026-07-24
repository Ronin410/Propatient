import { describe, it, expect, beforeAll, afterAll, vi } from 'vitest';
import { buildRecipeDocDefinition } from './recipePdf';
import type { Patient } from '../types';

// jsdom no implementa la carga real de <img> (no hay red de verdad en los
// tests): sin este stub, cada intento de logo (el del doctor o el
// respaldo de ProPatient, ver recipePdf.ts) se quedaría esperando el
// timeout completo de getBase64FromUrlWithTimeout en cada test. Con esto,
// "cargar" cualquier imagen falla de inmediato — se sigue ejerciendo el
// mismo camino real de manejo de errores del código, solo sin la espera.
beforeAll(() => {
  class FakeImage {
    onload: (() => void) | null = null;
    onerror: ((err?: unknown) => void) | null = null;
    crossOrigin = '';
    set src(_value: string) {
      setTimeout(() => this.onerror?.(new Error('jsdom no carga imágenes reales')), 0);
    }
  }
  vi.stubGlobal('Image', FakeImage);
});

afterAll(() => {
  vi.unstubAllGlobals();
});

// Recorre el árbol de contenido de pdfmake (mezcla de objetos, arrays y
// columnas anidadas) y concatena todos los `text` encontrados, para poder
// aserciones simples de "este texto aparece en la receta impresa".
function flattenText(node: unknown): string {
  if (node == null) return '';
  if (typeof node === 'string') return node;
  if (Array.isArray(node)) return node.map(flattenText).join(' ');
  if (typeof node === 'object') {
    const obj = node as Record<string, unknown>;
    let out = '';
    if ('text' in obj) out += flattenText(obj.text) + ' ';
    if ('columns' in obj) out += flattenText(obj.columns) + ' ';
    if ('stack' in obj) out += flattenText(obj.stack) + ' ';
    if ('table' in obj) out += flattenText((obj.table as Record<string, unknown>)?.body) + ' ';
    return out;
  }
  return '';
}

describe('buildRecipeDocDefinition', () => {
  const patient: Patient = {
    id: 1,
    firstName: 'Alejandro',
    lastName: 'Bueno',
    email: '',
    phone: '',
    birthDate: '1990-01-01',
    gender: 'M',
  };

  it('incluye el nombre y la cédula real del doctor (regresión: antes usaba firstName/lastName/professionalId, campos que el backend nunca envía)', async () => {
    const doc = await buildRecipeDocDefinition({
      doctorInfo: {
        fullName: 'Juan Pérez',
        licenseNumber: '12345678',
        medicalSpecialty: 'Pediatría',
        university: 'UNAM',
      },
      patientInfo: patient,
      dynamicNotes: {},
      recipeSections: {},
    });

    const flat = flattenText(doc.content);

    expect(flat).toContain('DR. JUAN PÉREZ');
    expect(flat).toContain('CÉDULA PROFESIONAL: 12345678');
    // El bloque de firma fluye como parte del contenido normal (no como
    // "footer" fijo al fondo de cada página, ver recipePdf.ts) y debe
    // aparecer una sola vez, con el nombre del doctor.
    expect(flat).toContain('FIRMA DEL MÉDICO');
    expect(doc.footer).toBeUndefined();
  });

  it('calcula la edad del paciente en vez de mostrar siempre N/A', async () => {
    const doc = await buildRecipeDocDefinition({
      doctorInfo: { fullName: 'Juan Pérez' },
      patientInfo: patient,
      dynamicNotes: {},
      recipeSections: {},
    });

    const flat = flattenText(doc.content);
    expect(flat).toMatch(/EDAD: \d+ AÑOS/);
  });

  it('imprime la leyenda de receta configurada por el doctor', async () => {
    const doc = await buildRecipeDocDefinition({
      doctorInfo: { fullName: 'Juan Pérez', recipeLegend: 'Favor de no automedicarse.' },
      patientInfo: patient,
      dynamicNotes: {},
      recipeSections: {},
    });

    const flat = flattenText(doc.content);
    expect(flat).toContain('Favor de no automedicarse.');
  });

  it('usa un respaldo genérico si el doctor no tiene fullName configurado', async () => {
    const doc = await buildRecipeDocDefinition({
      doctorInfo: {},
      patientInfo: patient,
      dynamicNotes: {},
      recipeSections: {},
    });

    const flat = flattenText(doc.content);
    expect(flat).toContain('MÉDICO GENERAL');
  });

  it('imprime el folio de la receta cuando el backend ya lo asignó', async () => {
    const doc = await buildRecipeDocDefinition({
      doctorInfo: { fullName: 'Juan Pérez' },
      patientInfo: patient,
      dynamicNotes: {},
      recipeSections: {},
      recipeNumber: 42,
    });

    const flat = flattenText(doc.content);
    expect(flat).toContain('NO. DE RECETA: 000042');
  });

  it('no imprime ningún folio si todavía no se pudo asignar uno', async () => {
    const doc = await buildRecipeDocDefinition({
      doctorInfo: { fullName: 'Juan Pérez' },
      patientInfo: patient,
      dynamicNotes: {},
      recipeSections: {},
    });

    const flat = flattenText(doc.content);
    expect(flat).not.toContain('NO. DE RECETA');
  });
});
