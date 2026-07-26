import React, { useState, useEffect } from 'react';
import api from '../api/axios';
import './SettingsNotes.scss'; // Conserva los estilos del modal anterior adaptados a página

interface NoteSectionConfig {
  id: string;
  label: string;
  placeholder: string;
  required: boolean;
}

export const SettingsNotes: React.FC = () => {
  const [configList, setConfigList] = useState<NoteSectionConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Al cargar la pantalla de ajustes, traemos la configuración desde la BD de Go
  useEffect(() => {
    const fetchTemplate = async () => {
      try {
        const response = await api.get('/doctor/template'); // Ajusta a tu endpoint real
        if (response.data && response.data.fields) {
          setConfigList(response.data.fields);
        } else {
          // Valores por defecto iniciales si la BD está vacía
          setConfigList([
            { id: 'subjetivo', label: 'Padecimiento Actual (Subjetivo)', placeholder: 'Lo que el paciente refiere...', required: false },
            { id: 'diagnostico', label: 'Diagnóstico', placeholder: 'Impresión diagnóstica...', required: true },
            { id: 'tratamiento', label: 'Plan y Tratamiento', placeholder: 'Receta e indicaciones...', required: true }
          ]);
        }
      } catch (error) {
        console.error("Error al obtener la configuración de la plantilla:", error);
      } finally {
        setLoading(false);
      }
    };
    fetchTemplate();
  }, []);

  const addField = () => {
    const newId = `custom_${Date.now()}`;
    setConfigList([...configList, { id: newId, label: 'Nuevo Apartado', placeholder: 'Escribe aquí...', required: false }]);
  };

  const removeField = (id: string) => {
    setConfigList(configList.filter(item => item.id !== id));
  };

  const updateField = (id: string, key: keyof NoteSectionConfig, value: string | boolean) => {
    setConfigList(configList.map(item => item.id === id ? { ...item, [key]: value } : item));
  };

  const handleSaveSettings = async () => {
    setSaving(true);
    try {
      // Guardamos la configuración de forma persistente en la Base de Datos
      await api.post('/doctor/template', { fields: configList });
      // Guardamos también una copia en localStorage como caché inmediato para evitar delays de red
      localStorage.setItem('doctor_notes_template', JSON.stringify(configList));
      alert("¡Estructura de notas guardada correctamente para todas las consultas!");
    } catch (error) {
      console.error("Error al guardar la plantilla:", error);
      alert("Hubo un error al guardar la configuración en el servidor.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <div className="settings-loading">Cargando configuraciones...</div>;

  return (
    <div className="settings-notes-container">
      <div className="settings-card">
        <h2>Configuración de Apartados en Consulta</h2>
        <p className="description">
          Define aquí los campos que aparecerán en las notas de evolución médica. Todas las consultas subsecuentes se verán idénticas respetando este formato.
        </p>
        
        <div className="fields-list">
          {configList.map((field, idx) => (
            <div className="field-row" key={field.id}>
              <span className="row-number">#{idx + 1}</span>
              <div className="input-group">
                <label>Nombre del Apartado</label>
                <input 
                  type="text" 
                  value={field.label} 
                  onChange={e => updateField(field.id, 'label', e.target.value)} 
                />
              </div>
              <div className="input-group">
                <label>Texto de ayuda (Placeholder)</label>
                <input 
                  type="text" 
                  value={field.placeholder} 
                  onChange={e => updateField(field.id, 'placeholder', e.target.value)} 
                />
              </div>
              <label className="checkbox-container">
                <input 
                  type="checkbox" 
                  checked={field.required} 
                  onChange={e => updateField(field.id, 'required', e.target.checked)} 
                />
                Obligatorio
              </label>
              <button type="button" className="btn-delete" onClick={() => removeField(field.id)}>
                <span className="material-icons-outlined">delete</span>
              </button>
            </div>
          ))}
        </div>

        <div className="settings-actions">
          <button type="button" className="btn-add" onClick={addField}>
            <span className="material-icons-outlined">add</span> Agregar Otro Apartado
          </button>
          <button type="button" className="btn-save-main" onClick={handleSaveSettings} disabled={saving}>
            {saving ? 'Guardando...' : 'Guardar Cambios Globales'}
          </button>
        </div>
      </div>
    </div>
  );
};