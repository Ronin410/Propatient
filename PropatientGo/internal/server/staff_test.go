package server_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaffInvite_AcceptAndLogin_FullLifecycle cubre el flujo completo: el
// doctor invita, la invitación se puede consultar públicamente, la persona
// crea su contraseña y puede iniciar sesión con un token que le da acceso a
// los datos del consultorio del doctor.
func TestStaffInvite_AcceptAndLogin_FullLifecycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_staff_lifecycle", "password123")
	doc.FullName = "Dra. Ejemplo"
	db.Save(&doc)
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/staff", docToken, map[string]any{
		"fullName": "Ana Secretaria", "email": "ana@consultorio.test",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var staff models.Staff
	require.NoError(t, db.Where("email = ?", "ana@consultorio.test").First(&staff).Error)
	require.NotEmpty(t, staff.InviteToken)
	require.False(t, staff.PasswordSet)

	w = doRequest(t, router, http.MethodGet, "/api/auth/staff-invite/"+staff.InviteToken, "", nil)
	require.Equal(t, http.StatusOK, w.Code)
	invite := decodeJSON(t, w)
	assert.Equal(t, "Ana Secretaria", invite["fullName"])
	assert.Equal(t, "Dra. Ejemplo", invite["doctorName"])

	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-invite/"+staff.InviteToken, "", map[string]any{
		"password": "claveSegura123",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// El token ya se consumió: no debe poder reutilizarse.
	w = doRequest(t, router, http.MethodGet, "/api/auth/staff-invite/"+staff.InviteToken, "", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login", "", map[string]any{
		"email": "ana@consultorio.test", "password": "claveSegura123",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	loginResp := decodeJSON(t, w)
	staffToken, _ := loginResp["token"].(string)
	require.NotEmpty(t, staffToken)
	assert.Equal(t, "Dra. Ejemplo", loginResp["doctorName"])

	// Con contraseña incorrecta, debe rechazar.
	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login", "", map[string]any{
		"email": "ana@consultorio.test", "password": "incorrecta",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// El token de personal ya debe poder listar los pacientes del consultorio del doctor.
	w = doRequest(t, router, http.MethodGet, "/api/patients", staffToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
}

// TestStaff_SharesDoctorsAgendaAndPatients confirma la premisa central: el
// personal opera sobre los MISMOS datos que su doctor (misma agenda, mismos
// pacientes), no una copia aislada.
func TestStaff_SharesDoctorsAgendaAndPatients(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_staff_shared", "password123")
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)
	staff := testutil.CreateTestStaff(t, db, doc.ID, "personal@consultorio.test", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	// El personal agenda una cita nueva (con paciente de registro rápido).
	w := doRequest(t, router, http.MethodPost, "/api/appointments", staffToken, map[string]any{
		"appointmentDateTime": "2026-08-01T10:00:00Z",
		"service":             "Consulta general",
		"patientFirstName":    "Paciente",
		"patientLastName":     "DePersonal",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// El doctor debe verla en su propia agenda.
	w = doRequest(t, router, http.MethodGet, "/api/appointments", docToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var appointments []map[string]any
	decodeJSONList(t, w, &appointments)
	require.Len(t, appointments, 1)

	// Y el personal debe ver también al paciente en el listado del consultorio.
	w = doRequest(t, router, http.MethodGet, "/api/patients", staffToken, nil)
	require.Equal(t, http.StatusOK, w.Code)
	patientsResp := decodeJSON(t, w)
	data, _ := patientsResp["data"].([]any)
	assert.Len(t, data, 1)
}

// TestStaff_CannotAccessDoctorOnlyRoutes es el test de seguridad central:
// una cuenta de personal debe recibir 403 en todo lo clínico/administrativo
// que el doctor eligió mantener fuera de su alcance.
func TestStaff_CannotAccessDoctorOnlyRoutes(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_staff_blocked", "password123")
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)
	staff := testutil.CreateTestStaff(t, db, doc.ID, "bloqueada@consultorio.test", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	// Paciente + cita creados por el doctor, para probar los endpoints de detalle.
	w := doRequest(t, router, http.MethodPost, "/api/patients", docToken, map[string]any{
		"firstName": "Con", "lastName": "Historial", "email": "con-historial@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))

	w = doRequest(t, router, http.MethodPost, "/api/appointments", docToken, map[string]any{
		"appointmentDateTime": "2026-08-02T10:00:00Z", "service": "Consulta", "patientId": patientID,
	})
	require.Equal(t, http.StatusCreated, w.Code)
	appointmentID := int(decodeJSON(t, w)["id"].(float64))

	pid := strconv.Itoa(patientID)
	aid := strconv.Itoa(appointmentID)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"historial médico combinado", http.MethodGet, "/api/patients/" + pid + "/history", nil},
		{"editar antecedentes", http.MethodPut, "/api/patients/" + pid + "/medical-history", map[string]any{"allergies": "x"}},
		{"detalle clínico de la cita", http.MethodGet, "/api/appointments/" + aid, nil},
		{"editar/finalizar consulta", http.MethodPut, "/api/appointments/" + aid, map[string]any{"diagnosis": "x"}},
		{"perfil del doctor", http.MethodGet, "/api/doctor/me", nil},
		{"actualizar licencia", http.MethodPost, "/api/user/update-license", map[string]any{}},
		{"invitar más personal", http.MethodPost, "/api/staff", map[string]any{"fullName": "x", "email": "otra@test.local"}},
		{"listar personal", http.MethodGet, "/api/staff", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequest(t, router, tc.method, tc.path, staffToken, tc.body)
			assert.Equal(t, http.StatusForbidden, w.Code, "el personal no debería poder acceder a: %s", tc.name)
		})
	}

	// Confirmamos que el MISMO doctor sí puede, para descartar que las rutas estén rotas para todos.
	w = doRequest(t, router, http.MethodGet, "/api/patients/"+pid+"/history", docToken, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestStaff_CannotSmuggleMedicalHistoryViaPatientUpdate verifica que el
// endpoint general de edición de paciente (PUT /api/patients/:id), que sí
// está abierto al personal, no sirva como puerta trasera para tocar
// antecedentes clínicos.
func TestStaff_CannotSmuggleMedicalHistoryViaPatientUpdate(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_staff_smuggle", "password123")
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)
	staff := testutil.CreateTestStaff(t, db, doc.ID, "smuggle@consultorio.test", "clave123456")
	staffToken := testutil.TokenForStaff(t, doc.ID, staff.ID, staff.Email)

	w := doRequest(t, router, http.MethodPost, "/api/patients", docToken, map[string]any{
		"firstName": "Paciente", "lastName": "Prueba", "email": "smuggle-patient@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code)
	patientID := int(decodeJSON(t, w)["id"].(float64))
	pid := strconv.Itoa(patientID)

	w = doRequest(t, router, http.MethodPut, "/api/patients/"+pid+"/medical-history", docToken, map[string]any{
		"allergies": "Penicilina (real)",
	})
	require.Equal(t, http.StatusOK, w.Code)

	// El personal edita el teléfono, pero intenta colar un cambio de antecedentes en el mismo payload.
	w = doRequest(t, router, http.MethodPut, "/api/patients/"+pid, staffToken, map[string]any{
		"firstName": "Paciente", "lastName": "Prueba", "phone": "5555555555",
		"medicalHistory": map[string]any{"allergies": "INYECTADO POR PERSONAL"},
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var history models.MedicalHistory
	require.NoError(t, db.Where("patient_id = ?", patientID).First(&history).Error)
	assert.Equal(t, "Penicilina (real)", history.Allergies, "el personal no debe poder modificar antecedentes vía PUT /patients/:id")
}

// TestStaff_InactiveAccountCannotLogin confirma que desactivar una cuenta de
// personal le corta el acceso de inmediato.
func TestStaff_InactiveAccountCannotLogin(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_staff_inactive", "password123")
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)
	staff := testutil.CreateTestStaff(t, db, doc.ID, "inactiva@consultorio.test", "clave123456")

	w := doRequest(t, router, http.MethodPut, "/api/staff/"+strconv.Itoa(int(staff.ID))+"/active", docToken, map[string]any{
		"active": false,
	})
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login", "", map[string]any{
		"email": staff.Email, "password": "clave123456",
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestStaffInvite_ExpiredTokenRejected asegura que una invitación vencida no
// se pueda usar para activar la cuenta.
func TestStaffInvite_ExpiredTokenRejected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_staff_expired", "password123")
	docToken := testutil.TokenFor(t, doc.ID, doc.Username)

	w := doRequest(t, router, http.MethodPost, "/api/staff", docToken, map[string]any{
		"fullName": "Invitación Vieja", "email": "vieja@consultorio.test",
	})
	require.Equal(t, http.StatusCreated, w.Code)

	var staff models.Staff
	require.NoError(t, db.Where("email = ?", "vieja@consultorio.test").First(&staff).Error)
	expired := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&staff).Update("invite_token_expires_at", expired).Error)

	w = doRequest(t, router, http.MethodGet, "/api/auth/staff-invite/"+staff.InviteToken, "", nil)
	assert.Equal(t, http.StatusGone, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-invite/"+staff.InviteToken, "", map[string]any{
		"password": "cualquierClave123",
	})
	assert.Equal(t, http.StatusGone, w.Code)
}
