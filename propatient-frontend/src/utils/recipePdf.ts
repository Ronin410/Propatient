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
  firstName?: string;
  lastName?: string;
  specialty?: string;
  professionalId?: string;
  institution?: string;
  address?: string;
  phone?: string;
  logoUrl?: string;
  serialCode?: string;
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
      { text: '\n' },
    ])
    .flat();

  const hasValidBase64 = doctorLogoBase64 && doctorLogoBase64.startsWith('data:image');

  return {
    pageSize: 'LETTER',
    pageMargins: [40, 40, 40, 80],
    defaultStyle: { font: 'Roboto' },
    content: [
      {
        columns: [
          hasValidBase64
            ? { image: doctorLogoBase64, width: 90, alignment: 'left' }
            : { text: 'MÉDICO GENERAL', fontSize: 14, bold: true, color: '#1a365d', margin: [0, 15, 0, 0] },
          [
            { text: `DR. ${doctorInfo?.firstName || ''} ${doctorInfo?.lastName || ''}`.toUpperCase(), style: 'doctorName' },
            { text: `${doctorInfo?.specialty || 'MÉDICO CIRUJANO Y PARTERO'}`, style: 'doctorSpecialty' },
            { text: `CÉDULA PROFESIONAL: ${doctorInfo?.professionalId || 'N/A'}`, style: 'doctorSub' },
            { text: `${doctorInfo?.institution || 'UNIVERSIDAD AUTÓNOMA DE SINALOA'}`, style: 'doctorSub' },
          ],
        ],
        columnGap: 20,
      },
      { canvas: [{ type: 'line', x1: 0, y1: 15, x2: 532, y2: 15, lineWidth: 2, lineColor: '#1a365d' }] },
      { text: '\n' },
      {
        style: 'patientTable',
        table: {
          widths: ['*', 120, 80],
          body: [
            [
              { text: `PACIENTE: ${patientInfo?.firstName || ''} ${patientInfo?.lastName || ''}`.toUpperCase(), style: 'tableCellBold' },
              { text: `EDAD: N/A AÑOS`, style: 'tableCell' },
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
          paddingTop: () => 6,
          paddingBottom: () => 6,
          paddingLeft: () => 8,
          paddingRight: () => 8,
        },
      },
      { text: '\n\n' },
      ...recipeContent,
    ],
    footer: () => {
      return {
        stack: [
          { text: '_______________________________________', alignment: 'center', color: '#cbd5e0' },
          { text: `DR. ${doctorInfo?.firstName || ''} ${doctorInfo?.lastName || ''}`.toUpperCase(), alignment: 'center', fontSize: 10, bold: true, color: '#2d3748', margin: [0, 2, 0, 2] },
          { text: 'FIRMA DEL MÉDICO', alignment: 'center', fontSize: 8, color: '#718096' },
          { text: `Dirección: ${doctorInfo?.address || 'Av. De la Clínica #123'} | Tel: ${doctorInfo?.phone || 'N/A'}`, alignment: 'center', fontSize: 8, color: '#718096', margin: [0, 4, 0, 0] },
        ],
        margin: [40, 0, 40, 0],
      };
    },
    styles: {
      doctorName: { fontSize: 16, bold: true, color: '#1a365d', alignment: 'right' },
      doctorSpecialty: { fontSize: 10, bold: true, color: '#4a5568', alignment: 'right' },
      doctorSub: { fontSize: 9, color: '#718096', alignment: 'right' },
      patientTable: { margin: [0, 5, 0, 15] },
      tableCell: { fontSize: 10, color: '#2d3748' },
      tableCellBold: { fontSize: 10, bold: true, color: '#1a365d' },
      sectionHeader: { fontSize: 11, bold: true, color: '#1a365d', margin: [0, 10, 0, 4], decoration: 'underline' },
      sectionBody: { fontSize: 11, color: '#2d3748', marginLeft: 10 },
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
  const fileName = `receta_${appointmentId}_doc_${doctorInfo?.serialCode || '0'}.pdf`;
  formData.append('recipe_pdf', blob, fileName);

  await api.post(`/appointments/${appointmentId}/save-recipe-pdf`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });

  return docDefinition;
}

export { pdfMake };
