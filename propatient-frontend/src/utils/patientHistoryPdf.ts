import { pdfMake } from './recipePdf';
import type { Patient } from '../types';

function formatDate(dateStr?: string): string {
  if (!dateStr) return 'N/A';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return 'N/A';
  return d.toLocaleDateString('es-MX');
}

// Genera y descarga (en el navegador, sin pasar por el backend) un PDF con
// los antecedentes médicos completos y la cronología de consultas de un
// paciente. A diferencia de la receta, este PDF no se guarda en el servidor:
// es solo para que el doctor se lleve una copia del expediente.
export function downloadPatientHistoryPDF(patient: Patient): void {
  const h = patient.medicalHistory;
  const appointments = [...(patient.appointments || [])].sort(
    (a, b) => new Date(b.appointmentDateTime).getTime() - new Date(a.appointmentDateTime).getTime()
  );

  const docDefinition: any = {
    pageSize: 'LETTER',
    pageMargins: [40, 40, 40, 40],
    defaultStyle: { font: 'Roboto' },
    content: [
      { text: 'Expediente Clínico', style: 'title' },
      { text: `${patient.firstName} ${patient.lastName}`, style: 'patientName' },
      {
        columns: [
          { text: `Teléfono: ${patient.phone || 'N/A'}`, style: 'meta' },
          { text: `Email: ${patient.email || 'N/A'}`, style: 'meta' },
          { text: `Nacimiento: ${formatDate(patient.birthDate)}`, style: 'meta' },
        ],
      },
      { canvas: [{ type: 'line', x1: 0, y1: 10, x2: 532, y2: 10, lineWidth: 1, lineColor: '#cbd5e0' }] },

      { text: 'Antecedentes Médicos', style: 'sectionTitle' },
      { text: `Alergias: ${h?.allergies || 'Ninguna registrada'}`, style: 'body' },
      { text: `Antecedentes patológicos: ${h?.pathological_history || 'Sin registros'}`, style: 'body' },
      { text: `Antecedentes no patológicos: ${h?.non_pathological_history || 'Sin registros'}`, style: 'body' },
      { text: `Antecedentes quirúrgicos: ${h?.surgical_history || 'Sin registros'}`, style: 'body' },
      { text: `Medicación actual: ${h?.current_medication || 'Sin registros'}`, style: 'body' },

      { text: 'Historial de Consultas', style: 'sectionTitle' },
      appointments.length === 0
        ? { text: 'No hay consultas registradas.', style: 'body' }
        : {
            table: {
              headerRows: 1,
              widths: ['auto', 'auto', '*', '*'],
              body: [
                [
                  { text: 'Fecha', style: 'tableHeader' },
                  { text: 'Estado', style: 'tableHeader' },
                  { text: 'Motivo / Diagnóstico', style: 'tableHeader' },
                  { text: 'Tratamiento', style: 'tableHeader' },
                ],
                ...appointments.map((app) => [
                  { text: formatDate(app.appointmentDateTime), style: 'tableCell' },
                  { text: app.status || 'N/A', style: 'tableCell' },
                  { text: [app.reason, app.diagnosis].filter(Boolean).join(' — ') || 'N/A', style: 'tableCell' },
                  { text: app.treatmentPlan || '—', style: 'tableCell' },
                ]),
              ],
            },
            layout: {
              hLineWidth: () => 0.5,
              vLineWidth: () => 0.5,
              hLineColor: () => '#cbd5e0',
              vLineColor: () => '#cbd5e0',
              paddingTop: () => 6,
              paddingBottom: () => 6,
              paddingLeft: () => 6,
              paddingRight: () => 6,
            },
          },
    ],
    styles: {
      title: { fontSize: 18, bold: true, color: '#1a365d', margin: [0, 0, 0, 4] },
      patientName: { fontSize: 14, bold: true, color: '#2d3748', margin: [0, 0, 0, 8] },
      meta: { fontSize: 9, color: '#718096' },
      sectionTitle: { fontSize: 12, bold: true, color: '#1a365d', margin: [0, 16, 0, 6], decoration: 'underline' },
      body: { fontSize: 10, color: '#2d3748', margin: [0, 2, 0, 2] },
      tableHeader: { fontSize: 9, bold: true, color: '#ffffff', fillColor: '#005073' },
      tableCell: { fontSize: 9, color: '#2d3748' },
    },
  };

  const fileName = `historial_${patient.firstName}_${patient.lastName}.pdf`.replace(/\s+/g, '_');
  pdfMake.createPdf(docDefinition).download(fileName);
}
