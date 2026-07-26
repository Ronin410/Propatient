package server_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFollowUp_SetShowAndDismiss cubre el ciclo completo del seguimiento
// programado: marcarlo, verlo en el dashboard, y descartarlo.
func TestFollowUp_SetShowAndDismiss(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doctor_followup", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Seguimiento", "email": "seguimiento@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-07-14T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	// Sin seguimiento marcado, no debe aparecer nada.
	w = doRequest(t, router, http.MethodGet, "/api/dashboard/follow-ups", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var emptyList []map[string]any
	decodeJSONList(t, w, &emptyList)
	assert.Empty(t, emptyList)

	followUpDate := time.Now().UTC().AddDate(0, 0, 3).Format(time.RFC3339)
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID), token, map[string]any{
		"followUpDate": followUpDate,
	})
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/dashboard/follow-ups", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var list []map[string]any
	decodeJSONList(t, w, &list)
	require.Len(t, list, 1)
	assert.Equal(t, float64(apptID), list[0]["id"])

	// Descartar: mandar followUpDate: null debe limpiarlo.
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID), token, map[string]any{
		"followUpDate": nil,
	})
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/dashboard/follow-ups", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var afterDismiss []map[string]any
	decodeJSONList(t, w, &afterDismiss)
	assert.Empty(t, afterDismiss, "el seguimiento descartado no debe seguir apareciendo")
}

// TestFollowUp_IsolatedPerDoctor confirma que el recordatorio de un doctor
// nunca se filtra hacia el dashboard de otro.
func TestFollowUp_IsolatedPerDoctor(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doctorA_followup", "passwordA123")
	docB := testutil.CreateTestDoctor(t, db, "doctorB_followup", "passwordB123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", tokenA, map[string]any{
		"firstName": "Paciente", "lastName": "DeA", "email": "pacientea_fu@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", tokenA, map[string]any{
		"appointmentDateTime": "2026-07-14T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	followUpDate := time.Now().UTC().AddDate(0, 0, 2).Format(time.RFC3339)
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID), tokenA, map[string]any{
		"followUpDate": followUpDate,
	})
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/dashboard/follow-ups", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var listB []map[string]any
	decodeJSONList(t, w, &listB)
	assert.Empty(t, listB, "el seguimiento de A no debe aparecer en el dashboard de B")
}

// TestFollowUp_ExcludesFarFutureDates confirma la ventana de 7 días: un
// seguimiento programado muy lejos en el futuro no debe saturar el dashboard
// todavía.
func TestFollowUp_ExcludesFarFutureDates(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doctor_followup_far", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Lejano", "email": "lejano@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-07-14T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	apptID := int(decodeJSON(t, w)["id"].(float64))

	farFuture := time.Now().UTC().AddDate(0, 0, 30).Format(time.RFC3339)
	w = doRequest(t, router, http.MethodPut, "/api/appointments/"+strconv.Itoa(apptID), token, map[string]any{
		"followUpDate": farFuture,
	})
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodGet, "/api/dashboard/follow-ups", token, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var farList []map[string]any
	decodeJSONList(t, w, &farList)
	assert.Empty(t, farList, "un seguimiento a 30 días no debe mostrarse todavía (ventana de 7 días)")
}
