import React, { useState } from 'react';
import api from '../api/axios';
import './ConsultationForm.scss';

interface Props {
  appointment: any;
  forceOpenPatientData?: boolean;
}

export const ConsultationForm: React.FC<Props> = ({ appointment, forceOpenPatientData }) => {
  const [formData, setFormData] = useState({
    subjective: '', // Lo que el paciente refiere
    objective: '',  // Hallazgos físicos (Signos vitales, exploración)
    diagnosis: '',  // Impresión diagnóstica
    treatmentPlan: '', // Receta y órdenes
  });

  const [isSaving, setIsUploading] = useState(false);

  const handleSave = async (isFinal: boolean) => {
    setIsUploading(true);
    try {
      const payload = {
        ...formData,
        status: isFinal ? 'COMPLETED' : 'IN_COURSE',
        appointmentDateTime: appointment.appointmentDateTime // Mantener consistencia
      };
      
      await api.put(`/appointments/${appointment.id}`, payload);
      
      if (isFinal) {
        alert("Consulta finalizada con éxito");
        window.location.href = '/inicio';
      }
    } catch (err) {
      alert("Error al guardar la consulta");
    } finally {
      setIsUploading(false);
    }
  };

  return (
    <div className="consultation-form-card card">
      <div className="form-section">
        <label>Nota Subjetiva (Padecimiento actual)</label>
        <textarea 
          placeholder="Describa los síntomas referidos por el paciente..."
          value={formData.subjective}
          onChange={(e) => setFormData({...formData, subjective: e.target.value})}
        />
      </div>

      <div className="form-section">
        <label>Exploración Objetiva (Signos Vitales / Hallazgos)</label>
        <textarea 
          placeholder="Tensión arterial, FC, Peso, Hallazgos físicos..."
          value={formData.objective}
          onChange={(e) => setFormData({...formData, objective: e.target.value})}
        />
      </div>

      <div className="form-section">
        <label>Diagnóstico</label>
        <input 
          type="text" 
          placeholder="Diagnóstico principal..."
          value={formData.diagnosis}
          onChange={(e) => setFormData({...formData, diagnosis: e.target.value})}
        />
      </div>

      <div className="form-section">
        <label>Plan de Tratamiento</label>
        <textarea 
          placeholder="Indicaciones terapéuticas y medicamentos..."
          value={formData.treatmentPlan}
          onChange={(e) => setFormData({...formData, treatmentPlan: e.target.value})}
        />
      </div>

      <div className="form-actions">
        <button className="btn-outline" onClick={() => handleSave(false)} disabled={isSaving}>Guardar Borrador</button>
        <button className="btn-primary" onClick={() => handleSave(true)} disabled={isSaving}>Finalizar Consulta</button>
      </div>
    </div>
  );
};