import pdfMake from 'pdfmake/build/pdfmake';
import * as pdfFonts from 'pdfmake/build/vfs_fonts';
import api from '../api/axios';
import type { Patient } from '../types';
import { toAbsoluteFileUrl } from './fileUrl';
import propatientLogo from '../assets/logo.png';

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
      try {
        const canvas = document.createElement('canvas');
        canvas.width = img.width;
        canvas.height = img.height;
        const ctx = canvas.getContext('2d');
        if (!ctx) throw new Error('No se pudo procesar el canvas');
        ctx.drawImage(img, 0, 0);
        // toDataURL truena con SecurityError si el servidor de la imagen no
        // manda Access-Control-Allow-Origin (canvas "manchado"/tainted) —
        // sin este try/catch esa excepción quedaba sin capturar dentro del
        // callback y la promesa nunca se resolvía NI se rechazaba, dejando
        // que el único respaldo fuera esperar el timeout completo de
        // getBase64FromUrlWithTimeout en vez de fallar de inmediato.
        resolve(canvas.toDataURL('image/png'));
      } catch (err) {
        reject(err);
      }
    };
    img.onerror = (error) => reject(error);
  });
}

// Con timeout: una imagen rota o un backend lento no debe dejar la receta
// esperando indefinidamente antes de caer al respaldo de texto.
function getBase64FromUrlWithTimeout(url: string, timeoutMs = 4000): Promise<string> {
  return Promise.race([
    getBase64FromUrl(url),
    new Promise<string>((_, reject) => setTimeout(() => reject(new Error('Tiempo de espera agotado al cargar la imagen')), timeoutMs)),
  ]);
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
  signatureUrl?: string;
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
  // Folio único asignado por el backend para ESTA cita (ver
  // GetOrAssignRecipeNumber) — nunca se calcula aquí, para que no se pueda
  // repetir entre recetas del mismo doctor. undefined si todavía no se pudo
  // consultar (ej. falla de red): la receta se sigue generando, solo sin
  // folio impreso.
  recipeNumber?: number;
}

// Arma el docDefinition de pdfmake para la receta médica.
export async function buildRecipeDocDefinition({
  doctorInfo,
  patientInfo,
  diagnosis,
  dynamicNotes,
  recipeSections,
  recipeNumber,
}: BuildRecipeParams): Promise<any> {
  // Logo a mostrar en el encabezado: el propio del doctor si lo tiene
  // configurado; si no, o si falla al cargarlo, el de ProPatient — nunca
  // el texto plano "MÉDICO GENERAL" salvo que ambas cargas de imagen
  // fallen (sin conexión, etc.), para que la receta siempre se vea con
  // una marca real en vez de un placeholder de texto.
  let logoBase64 = '';
  if (doctorInfo?.logoUrl) {
    const cleanFullUrl = toAbsoluteFileUrl(doctorInfo.logoUrl);
    try {
      logoBase64 = await getBase64FromUrlWithTimeout(cleanFullUrl);
    } catch (err) {
      // Se deja el detalle completo (URL + error real) en consola: la
      // causa típica de esto es que el servidor donde vive la imagen
      // (S3, u otro origen) no manda Access-Control-Allow-Origin para el
      // origen del frontend, lo cual solo se puede diagnosticar viendo
      // el error real, no solo "falló".
      console.error(`No se pudo cargar el logo del doctor desde ${cleanFullUrl}, se usará el de ProPatient:`, err);
      logoBase64 = '';
    }
  }
  if (!logoBase64 || !logoBase64.startsWith('data:image')) {
    try {
      logoBase64 = await getBase64FromUrlWithTimeout(propatientLogo);
    } catch (err) {
      console.error('No se pudo cargar el logo de respaldo de ProPatient, se usará texto:', err);
      logoBase64 = '';
    }
  }

  // Firma manuscrita (imagen) del doctor: es opcional — si no la configuró
  // o si falla al cargarla, el bloque de firma cae de vuelta a la línea +
  // nombre en texto que ya existía (ver signatureBlock más abajo), nunca
  // se rompe la generación de la receta por esto.
  let signatureBase64 = '';
  if (doctorInfo?.signatureUrl) {
    const cleanSignatureUrl = toAbsoluteFileUrl(doctorInfo.signatureUrl);
    try {
      signatureBase64 = await getBase64FromUrlWithTimeout(cleanSignatureUrl);
    } catch (err) {
      console.error(`No se pudo cargar la firma del doctor desde ${cleanSignatureUrl}, se usará el texto de respaldo:`, err);
      signatureBase64 = '';
    }
  }
  const hasValidSignature = signatureBase64 && signatureBase64.startsWith('data:image');

  const recipeContent = Object.keys(dynamicNotes)
    .filter((label) => recipeSections[label] !== false && dynamicNotes[label]?.trim() !== '')
    .map((label) => [
      { text: label.toUpperCase(), style: 'sectionHeader' },
      { text: dynamicNotes[label], style: 'sectionBody' },
    ])
    .flat();

  const hasValidBase64 = logoBase64 && logoBase64.startsWith('data:image');
  const age = calculateAge(patientInfo?.birthDate);
  const doctorFullName = doctorInfo?.fullName?.trim();

  const doctorSubLines = [
    { text: `${doctorInfo?.medicalSpecialty || 'Médico Cirujano y Partero'}`, style: 'doctorSpecialty' },
    { text: `CÉDULA PROFESIONAL: ${doctorInfo?.licenseNumber || 'N/A'}`, style: 'doctorSub' },
    ...(doctorInfo?.university?.trim() ? [{ text: doctorInfo.university, style: 'doctorSub' }] : []),
    // Folio de la receta: identifica a ESTA receta en particular contra
    // el resto de las que ha emitido el mismo doctor (ver
    // GetOrAssignRecipeNumber en el backend) — ayuda a detectar copias o
    // recetas alteradas, ya que dos recetas legítimas nunca comparten folio.
    ...(recipeNumber != null ? [{ text: `NO. DE RECETA: ${String(recipeNumber).padStart(6, '0')}`, style: 'doctorSub' }] : []),
  ];

  // La firma va como parte normal del contenido (no como "footer" de
  // pdfmake): un footer siempre se ancla al fondo de CADA página, así que
  // con una consulta corta dejaba un hueco enorme entre el diagnóstico y
  // la firma, y en una receta de dos páginas se repetía en ambas. Fluyendo
  // justo después del contenido, la receta se ve compacta cuando hay poco
  // texto y la firma solo aparece una vez, donde de verdad termina.
  const signatureBlock = {
    stack: [
      // Con firma-imagen configurada: la imagen reemplaza la línea
      // decorativa (ya "es" la firma). Sin ella: se conserva la línea en
      // blanco de siempre, para firmar a mano sobre el PDF impreso.
      hasValidSignature
        ? { image: signatureBase64, fit: [160, 60], alignment: 'center', margin: [0, 0, 0, 2] }
        : { text: '_______________________________________', alignment: 'center', color: '#cbd5e0' },
      { text: doctorFullName ? `DR. ${doctorFullName}`.toUpperCase() : 'MÉDICO GENERAL', alignment: 'center', fontSize: 10, bold: true, color: '#2d3748', margin: [0, 2, 0, 2] },
      { text: 'FIRMA DEL MÉDICO', alignment: 'center', fontSize: 8, color: '#718096' },
      { text: `Dirección: ${doctorInfo?.address || 'N/A'} | Tel: ${doctorInfo?.phone || 'N/A'}`, alignment: 'center', fontSize: 8, color: '#718096', margin: [0, 4, 0, 0] },
    ],
    margin: [0, 40, 0, 0],
  };

  return {
    pageSize: 'LETTER',
    pageMargins: [40, 32, 40, 40],
    defaultStyle: { font: 'Roboto' },
    content: [
      {
        columns: [
          hasValidBase64
            ? { image: logoBase64, fit: [60, 60], alignment: 'left' }
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
        margin: [0, 2, 0, 6],
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
      signatureBlock,
    ],
    styles: {
      doctorName: { fontSize: 17, bold: true, color: '#1a365d', alignment: 'right', margin: [0, 0, 0, 3] },
      doctorSpecialty: { fontSize: 12, bold: true, color: '#4a5568', alignment: 'right' },
      doctorSub: { fontSize: 11, color: '#718096', alignment: 'right' },
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
  // Folio: se pide/reserva ANTES de armar el PDF para poder imprimirlo ya
  // dentro de la receta (ver GetOrAssignRecipeNumber) — si el doctor vuelve
  // a generar la misma receta (ej. corrige una nota y le da "Generar
  // Receta" otra vez), el backend regresa el mismo folio ya asignado en
  // vez de uno nuevo, así que reintentar nunca "gasta" números de más.
  // Mejor esfuerzo: si falla la consulta, la receta se sigue generando,
  // solo sin folio impreso, para que un problema de red no bloquee la
  // consulta médica.
  let recipeNumber: number | undefined;
  try {
    const res = await api.get(`/appointments/${appointmentId}/recipe-number`);
    recipeNumber = res.data?.recipeNumber;
  } catch (err) {
    console.error('No se pudo obtener el folio de la receta, se generará sin folio:', err);
  }

  const docDefinition = await buildRecipeDocDefinition({
    doctorInfo,
    patientInfo,
    diagnosis: opts.diagnosis,
    dynamicNotes: opts.dynamicNotes,
    recipeSections: opts.recipeSections,
    recipeNumber,
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
