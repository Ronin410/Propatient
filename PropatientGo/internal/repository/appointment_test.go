package repository_test

import (
	"testing"
	"time"

	"propatient-api/internal/models"
	"propatient-api/internal/repository"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecNightClosure_MarksOverduePendingAsNoShow(t *testing.T) {
	db := testutil.SetupTestDB(t)
	doc := testutil.CreateTestDoctor(t, db, "doc_night_closure", "password123")

	patient := models.Patient{FirstName: "Paciente", LastName: "Vencido"}
	require.NoError(t, db.Create(&patient).Error)

	overdue := models.Appointment{
		PatientID:           patient.ID,
		DoctorID:            doc.ID,
		AppointmentDateTime: time.Now().UTC().Add(-2 * time.Hour),
		Status:              "PENDING",
	}
	require.NoError(t, db.Create(&overdue).Error)

	future := models.Appointment{
		PatientID:           patient.ID,
		DoctorID:            doc.ID,
		AppointmentDateTime: time.Now().UTC().Add(2 * time.Hour),
		Status:              "PENDING",
	}
	require.NoError(t, db.Create(&future).Error)

	require.NoError(t, repository.ExecNightClosure(db))

	var reloadedOverdue models.Appointment
	require.NoError(t, db.First(&reloadedOverdue, overdue.ID).Error)
	assert.Equal(t, "NOSHOW", reloadedOverdue.Status, "una cita PENDING cuya hora ya pasó debe quedar NOSHOW")

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
