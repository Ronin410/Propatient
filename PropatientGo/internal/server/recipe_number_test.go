package server_test

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"

	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recipeNumberTestEmailCounter garantiza un correo distinto en cada
// llamada: CreatePatient rechaza un segundo paciente con el mismo correo
// bajo el mismo doctor, y varios tests de este archivo crean más de una
// cita (y por lo tanto más de un paciente) para el mismo doctor.
var recipeNumberTestEmailCounter int64

// createTestAppointmentForRecipeNumber es el mismo par patient+appointment
// mínimo que ya usan los tests de storage_test.go, extraído aquí porque
// este archivo necesita crear varias citas por test.
func createTestAppointmentForRecipeNumber(t *testing.T, router *gin.Engine, token string) int {
	t.Helper()
	n := atomic.AddInt64(&recipeNumberTestEmailCounter, 1)
	w := doRequest(t, router, http.MethodPost, "/api/patients", token, map[string]any{
		"firstName": "Paciente", "lastName": "Folio", "email": "folio_" + strconv.FormatInt(n, 10) + "@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", token, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	return int(decodeJSON(t, w)["id"].(float64))
}

// TestRecipeNumber_FirstCall_AssignsSequentialNumber confirma el caso
// central: la primera vez que se pide el folio de una receta, se le asigna
// 1 (contador nuevo del doctor, ver Doctor.LastRecipeNumber), y una
// segunda cita del MISMO doctor recibe el folio 2 — nunca se repite.
func TestRecipeNumber_FirstCall_AssignsSequentialNumber(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_recipe_seq", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)

	appt1 := createTestAppointmentForRecipeNumber(t, router, token)
	appt2 := createTestAppointmentForRecipeNumber(t, router, token)

	w := doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(appt1)+"/recipe-number", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 1, decodeJSON(t, w)["recipeNumber"])

	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(appt2)+"/recipe-number", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 2, decodeJSON(t, w)["recipeNumber"])
}

// TestRecipeNumber_RepeatedCall_ReturnsSameNumber confirma que volver a
// pedir el folio de la MISMA cita (ej. el doctor regenera la receta tras
// corregir una nota) no "gasta" un número nuevo — regresa el que ya tenía.
func TestRecipeNumber_RepeatedCall_ReturnsSameNumber(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_recipe_repeat", "password123")
	token := testutil.TokenFor(t, doc.ID, doc.Username)
	appt := createTestAppointmentForRecipeNumber(t, router, token)

	w := doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(appt)+"/recipe-number", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	first := decodeJSON(t, w)["recipeNumber"]

	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(appt)+"/recipe-number", token, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	second := decodeJSON(t, w)["recipeNumber"]

	assert.Equal(t, first, second)

	var updated models.Appointment
	require.NoError(t, db.First(&updated, appt).Error)
	require.NotNil(t, updated.RecipeNumber)
	assert.EqualValues(t, first, *updated.RecipeNumber)
}

// TestRecipeNumber_DifferentDoctors_EachHasOwnSequence confirma que el
// contador es POR DOCTOR: dos doctores distintos, cada uno con su primera
// receta, ambos reciben folio 1 — no es una secuencia global.
func TestRecipeNumber_DifferentDoctors_EachHasOwnSequence(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doc_recipe_a", "password123")
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	apptA := createTestAppointmentForRecipeNumber(t, router, tokenA)

	docB := testutil.CreateTestDoctor(t, db, "doc_recipe_b", "password123")
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)
	apptB := createTestAppointmentForRecipeNumber(t, router, tokenB)

	w := doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(apptA)+"/recipe-number", tokenA, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 1, decodeJSON(t, w)["recipeNumber"])

	w = doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(apptB)+"/recipe-number", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 1, decodeJSON(t, w)["recipeNumber"])
}

// TestRecipeNumber_OtherDoctorsAppointment_Returns404 confirma que no se
// puede pedir/filtrar el folio de la cita de OTRO doctor.
func TestRecipeNumber_OtherDoctorsAppointment_Returns404(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	owner := testutil.CreateTestDoctor(t, db, "doc_recipe_owner", "password123")
	ownerToken := testutil.TokenFor(t, owner.ID, owner.Username)
	appt := createTestAppointmentForRecipeNumber(t, router, ownerToken)

	intruder := testutil.CreateTestDoctor(t, db, "doc_recipe_intruder", "password123")
	intruderToken := testutil.TokenFor(t, intruder.ID, intruder.Username)

	w := doRequest(t, router, http.MethodGet, "/api/appointments/"+strconv.Itoa(appt)+"/recipe-number", intruderToken, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
