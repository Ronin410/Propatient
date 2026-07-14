import pdfMake from 'pdfmake/build/pdfmake';
import * as pdfFonts from 'pdfmake/build/vfs_fonts';
import api, { BACKEND_ORIGIN } from '../api/axios';
import type { Patient } from '../types';

// Sincronización robusta para el bundle de Vite (movida desde ConsultationManager.tsx)
if (pdfFonts && (pdfFonts as any).pdfMake) {
  (pdfMake as any).vfs = (pdfFonts as any).pdfMake.vfs;
} else if ((pdfFonts as any).vfs) {
  (pdfMake as any).vfs = (pdfFonts as any).vfs;
}
if (typeof window !== 'undefined') {
  (window as any).pdfMake = pdfMake;
  (window as any).pdfMake.vfs = (pdfMake as any).vfs;
}

export function getBase64FromUrl(url: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.crossOrigin = 'anonymous'; // Enfoque nativo para evitar bloqueos XHR
    img.src = url;

    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = img.width;
      canvas.height = img.height;
      const ctx = canvas.getContext('2d');
      if (ctx) {
        ctx.drawImage(img, 0, 0);
        resolve(canvas.toDataURL('image/png'));
      } else {
        reject(new Error('No se pudo procesar el canvas'));
      }
    };
    img.onerror = (error) => reject(error);
  });
}

interface DoctorInfo {
  id?: number;
  fullName?: string;
  medicalSpecialty?: string;
  licenseNumber?: string;
  university?: string;
  address?: string;
  phone?: string;
  logoUrl?: string;
  recipeLegend?: string;
}

// Calcula la edad en años a partir de un birthDate "YYYY-MM-DD". Devuelve
// null si no hay fecha o no es parseable, para poder mostrar "N/A" en ese caso.
function calculateAge(birthDate?: string): number | null {
  if (!birthDate) return null;
  const parsed = new Date(birthDate);
  if (Number.isNaN(parsed.getTime())) return null;

  const today = new Date();
  let age = today.getFullYear() - parsed.getFullYear();
  const hasHadBirthdayThisYear =
    today.getMonth() > parsed.getMonth() ||
    (today.getMonth() === parsed.getMonth() && today.getDate() >= parsed.getDate());
  if (!hasHadBirthdayThisYear) age -= 1;
  return age;
}

interface BuildRecipeParams {
  doctorInfo: DoctorInfo;
  patientInfo?: Patient;
  diagnosis?: string;
  dynamicNotes: Record<string, string>;
  recipeSections: Record<string, boolean>;
}

// Arma el docDefinition de pdfmake para la receta médica.
export async function buildRecipeDocDefinition({
  doctorInfo,
  patientInfo,
  diagnosis,
  dynamicNotes,
  recipeSections,
}: BuildRecipeParams): Promise<any> {
  let doctorLogoBase64 = '';
  if (doctorInfo?.logoUrl) {
    try {
      const cleanFullUrl = doctorInfo.logoUrl.startsWith('http')
        ? doctorInfo.logoUrl
        : `${BACKEND_ORIGIN}${doctorInfo.logoUrl}`;
      doctorLogoBase64 = await getBase64FromUrl(cleanFullUrl);
    } catch (err) {
      console.error('No se pudo mapear el logo del doctor, usando respaldo de texto:', err);
      doctorLogoBase64 = ''; // Si falla, mantiene el texto de respaldo para que no truene la receta
    }
  }

  const recipeContent = Object.keys(dynamicNotes)
    .filter((label) => recipeSections[label] !== false && dynamicNotes[label]?.trim() !== '')
    .map((label) => [
      { text: label.toUpperCase(), style: 'sectionHeader' },
      { text: dynamicNotes[label], style: 'sectionBody' },
    ])
    .flat();

  const hasValidBase64 = doctorLogoBase64 && doctorLogoBase64.startsWith('data:image');
  const age = calculateAge(patientInfo?.birthDate);
  const doctorFullName = doctorInfo?.fullName?.trim();

  const doctorSubLines = [
    { text: `${doctorInfo?.medicalSpecialty || 'Médico Cirujano y Partero'}`, style: 'doctorSpecialty' },
    { text: `CÉDULA PROFESIONAL: ${doctorInfo?.licenseNumber || 'N/A'}`, style: 'doctorSub' },
    ...(doctorInfo?.university?.trim() ? [{ text: doctorInfo.university, style: 'doctorSub' }] : []),
  ];

  return {
    pageSize: 'LETTER',
    pageMargins: [40, 32, 40, 70],
    defaultStyle: { font: 'Roboto' },
    content: [
      {
        columns: [
          hasValidBase64
            ? { image: doctorLogoBase64, fit: [60, 60], alignment: 'left' }
            : { text: 'MÉDICO GENERAL', fontSize: 13, bold: true, color: '#1a365d' },
          [
            { text: doctorFullName ? `DR. ${doctorFullName}`.toUpperCase() : 'MÉDICO GENERAL', style: 'doctorName' },
            ...doctorSubLines,
          ],
        ],
        columnGap: 16,
      },
      {
        canvas: [{ type: 'line', x1: 0, y1: 0, x2: 532, y2: 0, lineWidth: 1.5, lineColor: '#1a365d' }],
        margin: [0, 10, 0, 12],
      },
      {
        style: 'patientTable',
        table: {
          widths: ['*', 120, 80],
          body: [
            [
              { text: `PACIENTE: ${patientInfo?.firstName || ''} ${patientInfo?.lastName || ''}`.toUpperCase(), style: 'tableCellBold' },
              { text: age !== null ? `EDAD: ${age} AÑOS` : 'EDAD: N/A', style: 'tableCell' },
              { text: `FECHA: ${new Date().toLocaleDateString()}`, style: 'tableCell', alignment: 'right' },
            ],
            [
              { text: `DIAGNÓSTICO: ${diagnosis || 'Sintomatología general'}`, style: 'tableCell', colSpan: 3 },
              {},
              {},
            ],
          ],
        },
        layout: {
          hLineWidth: () => 0.5,
          vLineWidth: () => 0.5,
          hLineColor: () => '#cbd5e0',
          vLineColor: () => '#cbd5e0',
          paddingTop: () => 7,
          paddingBottom: () => 7,
          paddingLeft: () => 10,
          paddingRight: () => 10,
          fillColor: (rowIndex: number) => (rowIndex === 0 ? '#f0f5fa' : null),
        },
      },
      ...recipeContent,
      ...(doctorInfo?.recipeLegend?.trim()
        ? [{ text: doctorInfo.recipeLegend, style: 'legend' }]
        : []),
    ],
    footer: () => {
      return {
        stack: [
          { text: '_______________________________________', alignment: 'center', color: '#cbd5e0' },
          { text: doctorFullName ? `DR. ${doctorFullName}`.toUpperCase() : 'MÉDICO GENERAL', alignment: 'center', fontSize: 10, bold: true, color: '#2d3748', margin: [0, 2, 0, 2] },
          { text: 'FIRMA DEL MÉDICO', alignment: 'center', fontSize: 8, color: '#718096' },
          { text: `Dirección: ${doctorInfo?.address || 'N/A'} | Tel: ${doctorInfo?.phone || 'N/A'}`, alignment: 'center', fontSize: 8, color: '#718096', margin: [0, 4, 0, 0] },
        ],
        margin: [40, 0, 40, 0],
      };
    },
    styles: {
      doctorName: { fontSize: 15, bold: true, color: '#1a365d', alignment: 'right', margin: [0, 0, 0, 3] },
      doctorSpecialty: { fontSize: 10, bold: true, color: '#4a5568', alignment: 'right' },
      doctorSub: { fontSize: 9, color: '#718096', alignment: 'right' },
      patientTable: { margin: [0, 0, 0, 10] },
      tableCell: { fontSize: 10, color: '#2d3748' },
      tableCellBold: { fontSize: 10, bold: true, color: '#1a365d' },
      sectionHeader: { fontSize: 11, bold: true, color: '#1a365d', margin: [0, 8, 0, 3], decoration: 'underline' },
      sectionBody: { fontSize: 11, color: '#2d3748', marginLeft: 10, marginBottom: 4 },
      legend: { fontSize: 8, italics: true, color: '#718096', margin: [0, 8, 0, 0] },
    },
  };
}

// Arma la receta, la sube al backend y devuelve el docDefinition (para poder
// imprimirla después sin tener que regenerarla). Usada tanto por el botón
// "Generar Receta" como por "Finalizar Consulta".
export async function generateAndSaveRecipePDF(
  doctorInfo: DoctorInfo,
  patientInfo: Patient | undefined,
  appointmentId: string,
  opts: { diagnosis?: string; dynamicNotes: Record<string, string>; recipeSections: Record<string, boolean> }
): Promise<any> {
  const docDefinition = await buildRecipeDocDefinition({
    doctorInfo,
    patientInfo,
    diagnosis: opts.diagnosis,
    dynamicNotes: opts.dynamicNotes,
    recipeSections: opts.recipeSections,
  });

  const pdfInstance = pdfMake.createPdf(docDefinition);
  const blob: Blob = await pdfInstance.getBlob();
  const formData = new FormData();
  const fileName = `receta_${appointmentId}_doc_${doctorInfo?.id || '0'}.pdf`;
  formData.append('recipe_pdf', blob, fileName);

  await api.post(`/appointments/${appointmentId}/save-recipe-pdf`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });

  return docDefinition;
}

export { pdfMake };
