package auth

import (
	"testing"
	"time"
)

// TestFormatSpanishDateTime_ConvertsUTCToLocalTimeZone cubre el bug
// reportado: una cita de las 10:00 a.m. hora del consultorio (UTC-7) se
// guarda en la base de datos como 17:00 UTC. Sin convertir de vuelta a la
// zona horaria del consultorio antes de formatear, el correo/WhatsApp
// mostraba "5:00 p.m." en vez de "10:00 a.m.".
func TestFormatSpanishDateTime_ConvertsUTCToLocalTimeZone(t *testing.T) {
	// 15 de julio 2026, 17:00 UTC == 10:00 a.m. en America/Mazatlan (UTC-7).
	utcTime := time.Date(2026, time.July, 15, 17, 0, 0, 0, time.UTC)

	got := FormatSpanishDateTime(utcTime)
	want := "15 de julio de 2026, 10:00 a.m."

	if got != want {
		t.Errorf("FormatSpanishDateTime(%v) = %q, want %q", utcTime, got, want)
	}
}

// TestFormatSpanishDateTime_HandlesDayBoundaryCrossing cubre el otro
// síntoma reportado: una cita agendada después de las 5pm hora local
// aparecía "un día después" — porque al cruzar la medianoche UTC, el día
// calendario también cambia y hay que reflejarlo en la fecha mostrada.
func TestFormatSpanishDateTime_HandlesDayBoundaryCrossing(t *testing.T) {
	// 16 de julio 2026, 01:00 UTC == 15 de julio, 6:00 p.m. en UTC-7.
	utcTime := time.Date(2026, time.July, 16, 1, 0, 0, 0, time.UTC)

	got := FormatSpanishDateTime(utcTime)
	want := "15 de julio de 2026, 06:00 p.m."

	if got != want {
		t.Errorf("FormatSpanishDateTime(%v) = %q, want %q", utcTime, got, want)
	}
}

// TestFormatSpanishDateTimeShort_UsesHoyMananaOrFallsBack cubre el texto
// corto usado en la notificación push (ver
// handlers.sendPublicBookingPush): "Hoy"/"Mañana" cuando aplica, y el
// formato largo de siempre para cualquier otra fecha. Se calcula "hoy" y
// "mañana" en tiempo de ejecución (no fechas fijas) para que el test no
// dependa de cuándo se corra.
func TestFormatSpanishDateTimeShort_UsesHoyMananaOrFallsBack(t *testing.T) {
	loc, err := time.LoadLocation(AppTimeZone)
	if err != nil {
		t.Fatalf("no se pudo cargar %s: %v", AppTimeZone, err)
	}
	now := time.Now().In(loc)

	today := time.Date(now.Year(), now.Month(), now.Day(), 10, 30, 0, 0, loc)
	if got, want := FormatSpanishDateTimeShort(today.UTC()), "Hoy, 10:30 a.m."; got != want {
		t.Errorf("FormatSpanishDateTimeShort(hoy) = %q, want %q", got, want)
	}

	tomorrow := today.AddDate(0, 0, 1)
	if got, want := FormatSpanishDateTimeShort(tomorrow.UTC()), "Mañana, 10:30 a.m."; got != want {
		t.Errorf("FormatSpanishDateTimeShort(mañana) = %q, want %q", got, want)
	}

	nextWeek := today.AddDate(0, 0, 7)
	got := FormatSpanishDateTimeShort(nextWeek.UTC())
	want := FormatSpanishDateTime(nextWeek.UTC())
	if got != want {
		t.Errorf("FormatSpanishDateTimeShort(en una semana) = %q, want el formato largo %q", got, want)
	}
}
