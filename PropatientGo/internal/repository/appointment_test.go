package repository_test

import (
	"testing"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/models"
	"propatient-api/internal/repository"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecNightClosure_OnlyTouchesPreviousDays cubre el comportamiento
// pedido: una cita PENDING de HOY nunca se marca NOSHOW automáticamente
// sin importar que su horario ya haya pasado (un paciente que llega tarde
// debe seguir apareciendo en el dashboard, accionable) — solo las citas
// PENDING de un día YA TERMINADO se cierran como NOSHOW.
func TestExecNightClosure_OnlyTouchesPreviousDays(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_night_closure", "password123")

	patient := models.Patient{FirstName: "Paciente", LastName: "Vencido"}
	require.NoError(t, db.Create(&patient).Error)

	loc, err := time.LoadLocation(auth.AppTimeZone)
	require.NoError(t, err)
	y, m, d := time.Now().In(loc).Date()
	startOfToday := time.Date(y, m, d, 0, 0, 0, 0, loc)

	yesterday := models.Appointment{
		PatientID:           patient.ID,
		DoctorID:            doc.ID,
		AppointmentDateTime: startOfToday.Add(-1 * time.Hour), // 11pm del día anterior
		Status:              "PENDING",
	}
	require.NoError(t, db.Create(&yesterday).Error)

	todayEarly := models.Appointment{
		PatientID:           patient.ID,
		DoctorID:            doc.ID,
		AppointmentDateTime: startOfToday.Add(1 * time.Hour), // hoy, aunque la hora ya haya pasado
		Status:              "PENDING",
	}
	require.NoError(t, db.Create(&todayEarly).Error)

	future := models.Appointment{
		PatientID:           patient.ID,
		DoctorID:            doc.ID,
		AppointmentDateTime: startOfToday.Add(48 * time.Hour),
		Status:              "PENDING",
	}
	require.NoError(t, db.Create(&future).Error)

	require.NoError(t, repository.ExecNightClosure(db))

	var reloadedYesterday models.Appointment
	require.NoError(t, db.First(&reloadedYesterday, yesterday.ID).Error)
	assert.Equal(t, "NOSHOW", reloadedYesterday.Status, "una cita PENDING de un día ya terminado debe quedar NOSHOW")

	var reloadedToday models.Appointment
	require.NoError(t, db.First(&reloadedToday, todayEarly.ID).Error)
	assert.Equal(t, "PENDING", reloadedToday.Status, "una cita PENDING de HOY no debe tocarse aunque su horario ya haya pasado")

	var reloadedFuture models.Appointment
	require.NoError(t, db.First(&reloadedFuture, future.ID).Error)
	assert.Equal(t, "PENDING", reloadedFuture.Status, "una cita PENDING futura no debe tocarse")
}

// TestExecNightClosure_DoesNotTouchOtherStatuses confirma que solo se
// tocan citas PENDING — CONFIRMED/COMPLETED/CANCELLED vencidas deben
// quedarse como están.
func TestExecNightClosure_DoesNotTouchOtherStatuses(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_night_closure_other", "password123")

	patient := models.Patient{FirstName: "Paciente", LastName: "Completado"}
	require.NoError(t, db.Create(&patient).Error)

	completed := models.Appointment{
		PatientID:           patient.ID,
		DoctorID:            doc.ID,
		AppointmentDateTime: time.Now().UTC().Add(-2 * time.Hour),
		Status:              "COMPLETED",
	}
	require.NoError(t, db.Create(&completed).Error)

	require.NoError(t, repository.ExecNightClosure(db))

	var reloaded models.Appointment
	require.NoError(t, db.First(&reloaded, completed.ID).Error)
	assert.Equal(t, "COMPLETED", reloaded.Status)
}
