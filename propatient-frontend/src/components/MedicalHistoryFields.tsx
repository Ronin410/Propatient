import React, { useState } from 'react';
import './MedicalHistoryFields.scss';

// Listas de opciones comunes por categoría de antecedente — usadas como
// chips en ChecklistField desde ConsultationManager.tsx y PatientDetail.tsx.
// Centralizadas aquí para que ambas pantallas ofrezcan exactamente las
// mismas opciones.
export const ALLERGY_OPTIONS = [
  'Penicilina', 'Otros antibióticos', 'AINEs / Aspirina', 'Sulfas',
  'Mariscos', 'Frutos secos', 'Látex', 'Polen / estacionales', 'Picadura de insectos',
];

export const PATHOLOGICAL_OPTIONS = [
  'Diabetes mellitus', 'Hipertensión arterial', 'Cardiopatías', 'Asma',
  'Hipotiroidismo', 'Hipertiroidismo', 'Cáncer', 'Epilepsia',
  'Enfermedad renal', 'VIH', 'Hepatitis',
];

export const SURGICAL_OPTIONS = [
  'Apendicectomía', 'Colecistectomía', 'Cesárea', 'Hernioplastia',
  'Fracturas', 'Amigdalectomía',
];

export const HEREDITARY_OPTIONS = [
  'Diabetes', 'Hipertensión', 'Cáncer', 'Cardiopatías',
  'Enfermedades renales', 'Enfermedades psiquiátricas',
];

// Compone las piezas seleccionadas (chips + texto libre) en el mismo string
// que ya guarda el backend (MedicalHistory sigue siendo texto plano, sin
// migración) — ver ChecklistField más abajo.
function composeChecklistText(chips: string[], otherText: string): string {
  const chipPart = chips.join(', ');
  const otherPart = otherText.trim();
  if (chipPart && otherPart) return `${chipPart}. ${otherPart}`;
  return chipPart || otherPart;
}

interface ChecklistFieldProps {
  value: string;
  onChange: (next: string) => void;
  options?: string[];
  noneLabel: string;
  otherLabel?: string;
  otherPlaceholder: string;
  disabled?: boolean;
}

// Campo de antecedente con un booleano "Ninguno/Sin antecedentes", chips de
// opciones comunes (multi-selección) y un cuadro de texto libre para lo que
// no esté en la lista. El valor que ya tenía el paciente (texto libre
// escrito antes de este cambio) se conserva tal cual en el cuadro de texto
// libre al abrir el campo por primera vez — nunca se descarta ni se intenta
// adivinar qué chip le corresponde, evita perder información ya registrada.
export const ChecklistField: React.FC<ChecklistFieldProps> = ({
  value,
  onChange,
  options = [],
  noneLabel,
  otherLabel = 'Otros / detalles',
  otherPlaceholder,
  disabled = false,
}) => {
  const [isNone, setIsNone] = useState(false);
  const [selectedChips, setSelectedChips] = useState<string[]>([]);
  const [otherText, setOtherText] = useState(value);

  const emit = (nextIsNone: boolean, nextChips: string[], nextOther: string) => {
    onChange(nextIsNone ? noneLabel : composeChecklistText(nextChips, nextOther));
  };

  const handleToggleNone = (checked: boolean) => {
    setIsNone(checked);
    emit(checked, selectedChips, otherText);
  };

  const handleToggleChip = (option: string) => {
    if (isNone) return;
    const next = selectedChips.includes(option)
      ? selectedChips.filter((c) => c !== option)
      : [...selectedChips, option];
    setSelectedChips(next);
    emit(isNone, next, otherText);
  };

  const handleOtherChange = (next: string) => {
    setOtherText(next);
    emit(isNone, selectedChips, next);
  };

  return (
    <div className="checklist-field">
      <label className="checkbox-toggle-row">
        <input
          type="checkbox"
          checked={isNone}
          disabled={disabled}
          onChange={(e) => handleToggleNone(e.target.checked)}
        />
        {noneLabel}
      </label>

      {options.length > 0 && (
        <div className="chip-options" aria-disabled={isNone || disabled}>
          {options.map((option) => (
            <button
              type="button"
              key={option}
              className={`chip-option ${selectedChips.includes(option) ? 'selected' : ''}`}
              disabled={isNone || disabled}
              onClick={() => handleToggleChip(option)}
            >
              {option}
            </button>
          ))}
        </div>
      )}

      <div className="checklist-other">
        <label>{otherLabel}</label>
        <textarea
          rows={2}
          placeholder={otherPlaceholder}
          value={otherText}
          disabled={isNone || disabled}
          onChange={(e) => handleOtherChange(e.target.value)}
        />
      </div>
    </div>
  );
};

const SMOKING_OPTIONS: { value: string; label: string }[] = [
  { value: 'no', label: 'No fuma' },
  { value: 'ocasional', label: 'Ocasional' },
  { value: 'activo', label: 'Fumador activo' },
  { value: 'exfumador', label: 'Ex-fumador' },
];

const ALCOHOL_OPTIONS: { value: string; label: string }[] = [
  { value: 'no', label: 'No consume' },
  { value: 'ocasional', label: 'Ocasional' },
  { value: 'frecuente', label: 'Frecuente' },
];

const EXERCISE_OPTIONS: { value: string; label: string }[] = [
  { value: 'si', label: 'Sí' },
  { value: 'no', label: 'No' },
];

const DIET_OPTIONS: { value: string; label: string }[] = [
  { value: 'balanceada', label: 'Balanceada' },
  { value: 'alta_grasas_azucares', label: 'Alta en grasas/azúcares' },
  { value: 'vegetariana', label: 'Vegetariana' },
  { value: 'vegana', label: 'Vegana' },
  { value: 'otra', label: 'Otra' },
];

interface HabitsLifestyleFieldProps {
  value: string;
  onChange: (next: string) => void;
  disabled?: boolean;
}

// Antecedentes no patológicos / hábitos: en vez de un solo cuadro de texto,
// cuatro selectores para lo más consultado en cada nota (tabaquismo,
// alcohol, actividad física, alimentación) más un cuadro de texto libre
// para cualquier detalle adicional (donde también se conserva el texto ya
// registrado antes de este cambio).
export const HabitsLifestyleField: React.FC<HabitsLifestyleFieldProps> = ({ value, onChange, disabled = false }) => {
  const [smoking, setSmoking] = useState('');
  const [alcohol, setAlcohol] = useState('');
  const [exercise, setExercise] = useState('');
  const [diet, setDiet] = useState('');
  const [otherText, setOtherText] = useState(value);

  const emit = (s: string, a: string, e: string, d: string, other: string) => {
    const parts: string[] = [];
    if (s) parts.push(`Tabaquismo: ${SMOKING_OPTIONS.find((o) => o.value === s)?.label}`);
    if (a) parts.push(`Alcohol: ${ALCOHOL_OPTIONS.find((o) => o.value === a)?.label}`);
    if (e) parts.push(`Actividad física: ${EXERCISE_OPTIONS.find((o) => o.value === e)?.label}`);
    if (d) parts.push(`Alimentación: ${DIET_OPTIONS.find((o) => o.value === d)?.label}`);
    const structured = parts.join('. ');
    const otherPart = other.trim();
    const composed = structured && otherPart ? `${structured}. ${otherPart}` : structured || otherPart;
    onChange(composed);
  };

  return (
    <div className="habits-lifestyle-field">
      <div className="form-grid">
        <div className="form-group">
          <label>Tabaquismo</label>
          <select
            value={smoking}
            disabled={disabled}
            onChange={(e) => { setSmoking(e.target.value); emit(e.target.value, alcohol, exercise, diet, otherText); }}
          >
            <option value="">Sin especificar</option>
            {SMOKING_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <div className="form-group">
          <label>Consumo de alcohol</label>
          <select
            value={alcohol}
            disabled={disabled}
            onChange={(e) => { setAlcohol(e.target.value); emit(smoking, e.target.value, exercise, diet, otherText); }}
          >
            <option value="">Sin especificar</option>
            {ALCOHOL_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <div className="form-group">
          <label>Actividad física</label>
          <select
            value={exercise}
            disabled={disabled}
            onChange={(e) => { setExercise(e.target.value); emit(smoking, alcohol, e.target.value, diet, otherText); }}
          >
            <option value="">Sin especificar</option>
            {EXERCISE_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <div className="form-group">
          <label>Alimentación</label>
          <select
            value={diet}
            disabled={disabled}
            onChange={(e) => { setDiet(e.target.value); emit(smoking, alcohol, exercise, e.target.value, otherText); }}
          >
            <option value="">Sin especificar</option>
            {DIET_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
      </div>
      <div className="checklist-other">
        <label>Otros hábitos / notas</label>
        <textarea
          rows={2}
          placeholder="Cualquier otro detalle relevante..."
          value={otherText}
          disabled={disabled}
          onChange={(e) => { setOtherText(e.target.value); emit(smoking, alcohol, exercise, diet, e.target.value); }}
        />
      </div>
    </div>
  );
};

const CONTRACEPTIVE_OPTIONS: { value: string; label: string }[] = [
  { value: 'ninguno', label: 'Ninguno' },
  { value: 'hormonal_oral', label: 'Hormonal oral' },
  { value: 'diu', label: 'DIU' },
  { value: 'preservativo', label: 'Preservativo' },
  { value: 'inyectable', label: 'Inyectable' },
  { value: 'implante', label: 'Implante subdérmico' },
  { value: 'otro', label: 'Otro' },
];

interface GynecoObstetricFieldProps {
  value: string;
  onChange: (next: string) => void;
  disabled?: boolean;
}

// Antecedentes ginecoobstétricos: booleano "No aplica" + campos numéricos y
// selector para lo que se pregunta en cada consulta (menarca, FUM, ciclos,
// fórmula obstétrica G/P/C/A, método anticonceptivo), más texto libre para
// cualquier otra nota (donde se conserva el texto ya registrado antes de
// este cambio).
export const GynecoObstetricField: React.FC<GynecoObstetricFieldProps> = ({ value, onChange, disabled = false }) => {
  const [notApplicable, setNotApplicable] = useState(false);
  const [menarca, setMenarca] = useState('');
  const [fum, setFum] = useState('');
  const [ciclos, setCiclos] = useState('');
  const [gestas, setGestas] = useState('');
  const [partos, setPartos] = useState('');
  const [cesareas, setCesareas] = useState('');
  const [abortos, setAbortos] = useState('');
  const [method, setMethod] = useState('');
  const [otherText, setOtherText] = useState(value);

  const emit = (
    na: boolean, men: string, f: string, c: string,
    g: string, p: string, ces: string, ab: string, m: string, other: string
  ) => {
    if (na) { onChange('No aplica'); return; }
    const parts: string[] = [];
    if (men.trim()) parts.push(`Menarca: ${men.trim()} años`);
    if (f) parts.push(`FUM: ${f}`);
    if (c.trim()) parts.push(`Ciclos: ${c.trim()}`);
    if (g.trim() || p.trim() || ces.trim() || ab.trim()) {
      parts.push(`G:${g.trim() || '0'} P:${p.trim() || '0'} C:${ces.trim() || '0'} A:${ab.trim() || '0'}`);
    }
    if (m) parts.push(`Método anticonceptivo: ${CONTRACEPTIVE_OPTIONS.find((o) => o.value === m)?.label}`);
    const structured = parts.join('. ');
    const otherPart = other.trim();
    onChange(structured && otherPart ? `${structured}. ${otherPart}` : structured || otherPart);
  };

  return (
    <div className="gyneco-obstetric-field">
      <label className="checkbox-toggle-row">
        <input
          type="checkbox"
          checked={notApplicable}
          disabled={disabled}
          onChange={(e) => {
            setNotApplicable(e.target.checked);
            emit(e.target.checked, menarca, fum, ciclos, gestas, partos, cesareas, abortos, method, otherText);
          }}
        />
        No aplica
      </label>

      <fieldset disabled={notApplicable || disabled} className="gyneco-grid">
        <div className="form-group">
          <label>Menarca (edad)</label>
          <input
            type="number" min={0} max={99} placeholder="Ej: 12"
            value={menarca}
            onChange={(e) => { setMenarca(e.target.value); emit(notApplicable, e.target.value, fum, ciclos, gestas, partos, cesareas, abortos, method, otherText); }}
          />
        </div>
        <div className="form-group">
          <label>FUM (última menstruación)</label>
          <input
            type="date"
            value={fum}
            onChange={(e) => { setFum(e.target.value); emit(notApplicable, menarca, e.target.value, ciclos, gestas, partos, cesareas, abortos, method, otherText); }}
          />
        </div>
        <div className="form-group">
          <label>Ciclos</label>
          <input
            type="text" placeholder="Ej: 28/5"
            value={ciclos}
            onChange={(e) => { setCiclos(e.target.value); emit(notApplicable, menarca, fum, e.target.value, gestas, partos, cesareas, abortos, method, otherText); }}
          />
        </div>
        <div className="form-group">
          <label>Método anticonceptivo</label>
          <select
            value={method}
            onChange={(e) => { setMethod(e.target.value); emit(notApplicable, menarca, fum, ciclos, gestas, partos, cesareas, abortos, e.target.value, otherText); }}
          >
            <option value="">Sin especificar</option>
            {CONTRACEPTIVE_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <div className="form-group gpca-group">
          <label>Fórmula obstétrica (G / P / C / A)</label>
          <div className="gpca-inputs">
            <input type="number" min={0} placeholder="G" value={gestas} onChange={(e) => { setGestas(e.target.value); emit(notApplicable, menarca, fum, ciclos, e.target.value, partos, cesareas, abortos, method, otherText); }} />
            <input type="number" min={0} placeholder="P" value={partos} onChange={(e) => { setPartos(e.target.value); emit(notApplicable, menarca, fum, ciclos, gestas, e.target.value, cesareas, abortos, method, otherText); }} />
            <input type="number" min={0} placeholder="C" value={cesareas} onChange={(e) => { setCesareas(e.target.value); emit(notApplicable, menarca, fum, ciclos, gestas, partos, e.target.value, abortos, method, otherText); }} />
            <input type="number" min={0} placeholder="A" value={abortos} onChange={(e) => { setAbortos(e.target.value); emit(notApplicable, menarca, fum, ciclos, gestas, partos, cesareas, e.target.value, method, otherText); }} />
          </div>
        </div>
      </fieldset>

      <div className="checklist-other">
        <label>Otros / notas</label>
        <textarea
          rows={2}
          placeholder="Cualquier otro detalle relevante..."
          value={otherText}
          disabled={notApplicable || disabled}
          onChange={(e) => { setOtherText(e.target.value); emit(notApplicable, menarca, fum, ciclos, gestas, partos, cesareas, abortos, method, e.target.value); }}
        />
      </div>
    </div>
  );
};
