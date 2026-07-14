import { describe, it, expect } from 'vitest';
import { buildRecipeDocDefinition } from './recipePdf';
import type { Patient } from '../types';

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
    const footerFlat = flattenText(typeof doc.footer === 'function' ? doc.footer() : doc.footer);

    expect(flat).toContain('DR. JUAN PÉREZ');
    expect(flat).toContain('CÉDULA PROFESIONAL: 12345678');
    expect(footerFlat).toContain('DR. JUAN PÉREZ');
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
});
