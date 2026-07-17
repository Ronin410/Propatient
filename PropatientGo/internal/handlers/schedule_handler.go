package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"propatient-api/internal/auth"
	"propatient-api/internal/models"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// timeRange es un bloque de horas en formato "HH:MM" (24h), usado tanto
// para la ventana laboral de un día como para cada descanso dentro de ella.
type timeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// dayHours es el horario de un solo día de la semana: si el doctor atiende
// ese día, su ventana laboral, y los descansos a excluir dentro de ella
// (ej. "de 2 a 3 no trabajo").
type dayHours struct {
	Enabled bool        `json:"enabled"`
	Start   string      `json:"start"`
	End     string      `json:"end"`
	Breaks  []timeRange `json:"breaks"`
}

// weekSchedule es el horario completo del consultorio, un dayHours por
// cada día de la semana. Se guarda tal cual como JSON en
// models.DoctorSchedule.Days.
type weekSchedule struct {
	Sunday    dayHours `json:"sunday"`
	Monday    dayHours `json:"monday"`
	Tuesday   dayHours `json:"tuesday"`
	Wednesday dayHours `json:"wednesday"`
	Thursday  dayHours `json:"thursday"`
	Friday    dayHours `json:"friday"`
	Saturday  dayHours `json:"saturday"`
}

// dayHoursFor devuelve el dayHours correspondiente a un time.Weekday
// (0=domingo … 6=sábado, igual que Go y JS).
func dayHoursFor(week weekSchedule, day time.Weekday) dayHours {
	switch day {
	case time.Sunday:
		return week.Sunday
	case time.Monday:
		return week.Monday
	case time.Tuesday:
		return week.Tuesday
	case time.Wednesday:
		return week.Wednesday
	case time.Thursday:
		return week.Thursday
	case time.Friday:
		return week.Friday
	default:
		return week.Saturday
	}
}

var spanishWeekdays = map[time.Weekday]string{
	time.Sunday:    "domingos",
	time.Monday:    "lunes",
	time.Tuesday:   "martes",
	time.Wednesday: "miércoles",
	time.Thursday:  "jueves",
	time.Friday:    "viernes",
	time.Saturday:  "sábados",
}

// parseHHMM convierte "HH:MM" a minutos desde medianoche. El segundo valor
// es false si el formato no es válido — nunca hace panic con una hora mal
// formada, para que un dato corrupto/vacío no tumbe la validación de citas.
func parseHHMM(value string) (int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// GetDoctorSchedule devuelve el horario configurado del consultorio del
// doctor autenticado. Sin RequireDoctorRole: el personal también puede
// consultarlo (y guardarlo, ver SaveDoctorSchedule) — a diferencia del
// resto de /doctor/*, este horario es una herramienta operativa del día a
// día, no un dato exclusivo del doctor. Si nunca se configuró, devuelve
// "configured: false" con todos los días deshabilitados (no un error) para
// que el frontend arranque el formulario desde cero.
func GetDoctorSchedule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var schedule models.DoctorSchedule
		if err := db.Where("doctor_id = ?", doctorID).First(&schedule).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"configured": false, "days": weekSchedule{}})
			return
		}

		var week weekSchedule
		if err := json.Unmarshal(schedule.Days, &week); err != nil {
			c.JSON(http.StatusOK, gin.H{"configured": false, "days": weekSchedule{}})
			return
		}

		c.JSON(http.StatusOK, gin.H{"configured": true, "days": week})
	}
}

type saveScheduleRequest struct {
	Days weekSchedule `json:"days" binding:"required"`
}

// validateWeekSchedule confirma que las horas capturadas tengan sentido
// antes de guardarlas: formato válido, inicio antes que fin, y que cada
// descanso quede dentro de la ventana laboral del día. Se valida al
// GUARDAR el horario (no en cada cita) para que un doctor nunca pueda
// dejar guardada una configuración imposible de cumplir.
func validateWeekSchedule(week weekSchedule) error {
	days := []struct {
		name string
		d    dayHours
	}{
		{"domingo", week.Sunday}, {"lunes", week.Monday}, {"martes", week.Tuesday},
		{"miércoles", week.Wednesday}, {"jueves", week.Thursday}, {"viernes", week.Friday},
		{"sábado", week.Saturday},
	}

	for _, entry := range days {
		if !entry.d.Enabled {
			continue
		}
		start, okStart := parseHHMM(entry.d.Start)
		end, okEnd := parseHHMM(entry.d.End)
		if !okStart || !okEnd {
			return fmt.Errorf("el horario del %s no es válido", entry.name)
		}
		if start >= end {
			return fmt.Errorf("en %s, la hora de inicio debe ser antes que la de fin", entry.name)
		}
		for _, brk := range entry.d.Breaks {
			bStart, okBStart := parseHHMM(brk.Start)
			bEnd, okBEnd := parseHHMM(brk.End)
			if !okBStart || !okBEnd || bStart >= bEnd {
				return fmt.Errorf("un descanso del %s no es válido", entry.name)
			}
			if bStart < start || bEnd > end {
				return fmt.Errorf("un descanso del %s debe caer dentro de su horario laboral", entry.name)
			}
		}
	}
	return nil
}

// SaveDoctorSchedule crea o actualiza el horario del consultorio (upsert,
// mismo patrón que SaveDoctorTemplate). Sin RequireDoctorRole, ver
// GetDoctorSchedule.
func SaveDoctorSchedule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var req saveScheduleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := validateWeekSchedule(req.Days); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		daysJSON, err := json.Marshal(req.Days)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar el horario"})
			return
		}

		var schedule models.DoctorSchedule
		err = db.Where("doctor_id = ?", doctorID).First(&schedule).Error
		if err == gorm.ErrRecordNotFound {
			schedule = models.DoctorSchedule{DoctorID: doctorID, Days: datatypes.JSON(daysJSON)}
			if err := db.Create(&schedule).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar el horario"})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar el horario"})
			return
		} else {
			schedule.Days = datatypes.JSON(daysJSON)
			if err := db.Save(&schedule).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar el horario"})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"configured": true, "days": req.Days})
	}
}

// ValidateAppointmentAgainstSchedule confirma que "when" (guardado en UTC,
// igual que el resto de la app) cae dentro del horario laboral configurado
// por el doctor: día habilitado, dentro de su ventana de inicio/fin, y
// fuera de cualquier descanso. Se usa al crear una cita (interna o
// pública) y al reprogramar una existente — ver appointment_handler.go y
// public_handler.go. Si el doctor nunca configuró un horario, no hay
// ninguna restricción (mismo criterio de "opcional" que el resto de las
// integraciones de esta app: no romper el flujo de nadie que no lo use).
func ValidateAppointmentAgainstSchedule(db *gorm.DB, doctorID uint, when time.Time) error {
	var schedule models.DoctorSchedule
	if err := db.Where("doctor_id = ?", doctorID).First(&schedule).Error; err != nil {
		return nil
	}

	var week weekSchedule
	if err := json.Unmarshal(schedule.Days, &week); err != nil {
		return nil
	}

	loc, err := time.LoadLocation(auth.AppTimeZone)
	if err != nil {
		return nil
	}
	local := when.In(loc)

	day := dayHoursFor(week, local.Weekday())
	if !day.Enabled {
		return fmt.Errorf("el consultorio no atiende los %s", spanishWeekdays[local.Weekday()])
	}

	minutes := local.Hour()*60 + local.Minute()
	startMin, okStart := parseHHMM(day.Start)
	endMin, okEnd := parseHHMM(day.End)
	if !okStart || !okEnd || minutes < startMin || minutes >= endMin {
		return fmt.Errorf("el horario de atención ese día es de %s a %s", day.Start, day.End)
	}

	for _, brk := range day.Breaks {
		bStart, okBStart := parseHHMM(brk.Start)
		bEnd, okBEnd := parseHHMM(brk.End)
		if okBStart && okBEnd && minutes >= bStart && minutes < bEnd {
			return fmt.Errorf("el consultorio no atiende de %s a %s ese día", brk.Start, brk.End)
		}
	}

	return nil
}
