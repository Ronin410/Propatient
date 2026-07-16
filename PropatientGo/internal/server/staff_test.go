package server_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
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

// TestStaffPasswordReset_UnknownEmail_ReturnsGenericMessage confirma que el
// endpoint no revela si un correo existe o no como cuenta de personal.
func TestStaffPasswordReset_UnknownEmail_ReturnsGenericMessage(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodPost, "/api/auth/staff-password-reset/request", "", map[string]any{
		"email": "no_existe@consultorio.test",
	})
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON(t, w)
	assert.Contains(t, body["message"], "Si el correo está registrado")
}

// TestStaffPasswordReset_FullLifecycle cubre el flujo completo: se solicita
// el reseteo, se genera un token en la cuenta, se usa para poner una
// contraseña nueva, y la contraseña vieja deja de servir.
func TestStaffPasswordReset_FullLifecycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_staff_reset", "password123")
	staff := testutil.CreateTestStaff(t, db, doc.ID, "reset@consultorio.test", "claveVieja123")

	w := doRequest(t, router, http.MethodPost, "/api/auth/staff-password-reset/request", "", map[string]any{
		"email": staff.Email,
	})
	require.Equal(t, http.StatusOK, w.Code)

	var reloaded models.Staff
	require.NoError(t, db.First(&reloaded, staff.ID).Error)
	require.NotEmpty(t, reloaded.PasswordResetToken, "debe haberse generado un token de reseteo")

	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-password-reset/"+reloaded.PasswordResetToken, "", map[string]any{
		"password": "claveNueva456",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// El token ya se consumió: no debe poder reutilizarse.
	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-password-reset/"+reloaded.PasswordResetToken, "", map[string]any{
		"password": "otraClave789",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)

	// La contraseña vieja ya no funciona.
	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login", "", map[string]any{
		"email": staff.Email, "password": "claveVieja123",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// La contraseña nueva sí funciona.
	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login", "", map[string]any{
		"email": staff.Email, "password": "claveNueva456",
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestStaffPasswordReset_ExpiredTokenRejected asegura que un link de
// reseteo vencido no se pueda usar.
func TestStaffPasswordReset_ExpiredTokenRejected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	doc := testutil.CreateTestDoctor(t, db, "doc_staff_reset_expired", "password123")
	staff := testutil.CreateTestStaff(t, db, doc.ID, "reset_expired@consultorio.test", "claveVieja123")

	expired := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, db.Model(&staff).Updates(map[string]interface{}{
		"password_reset_token":            "token-de-prueba-vencido",
		"password_reset_token_expires_at": expired,
	}).Error)

	w := doRequest(t, router, http.MethodPost, "/api/auth/staff-password-reset/token-de-prueba-vencido", "", map[string]any{
		"password": "claveNueva456",
	})
	assert.Equal(t, http.StatusGone, w.Code)
}

// TestStaffPasswordReset_InvalidTokenRejected confirma que un token
// inventado (nunca emitido) se rechaza.
func TestStaffPasswordReset_InvalidTokenRejected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	w := doRequest(t, router, http.MethodPost, "/api/auth/staff-password-reset/token-que-nunca-existio", "", map[string]any{
		"password": "claveNueva456",
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestStaff_SecondDoctorCanInviteSameEmail confirma el caso central de la
// clínica: un segundo doctor puede invitar un correo que otro doctor ya
// invitó (antes daba 409, porque Staff.Email era único globalmente) — se
// crea solo el vínculo nuevo, reutilizando la cuenta/contraseña existente.
func TestStaff_SecondDoctorCanInviteSameEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doc_clinic_a", "password123")
	docB := testutil.CreateTestDoctor(t, db, "doc_clinic_b", "password123")
	docA.FullName = "Dra. A"
	docB.FullName = "Dr. B"
	require.NoError(t, db.Save(&docA).Error)
	require.NoError(t, db.Save(&docB).Error)
	tokenA := testutil.TokenFor(t, docA.ID, docA.Username)
	tokenB := testutil.TokenFor(t, docB.ID, docB.Username)

	w := doRequest(t, router, http.MethodPost, "/api/staff", tokenA, map[string]any{
		"fullName": "Recepcionista Compartida", "email": "compartida@clinica.test",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var staff models.Staff
	require.NoError(t, db.Where("email = ?", "compartida@clinica.test").First(&staff).Error)
	require.NoError(t, db.Model(&staff).Updates(map[string]interface{}{
		"password_hash": mustHash(t, "claveCompartida1"),
		"password_set":  true,
	}).Error)

	// Doctor B invita el MISMO correo: no debe dar 409 (antes lo daba).
	w = doRequest(t, router, http.MethodPost, "/api/staff", tokenB, map[string]any{
		"fullName": "Recepcionista Compartida", "email": "compartida@clinica.test",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// Invitar la misma persona DE NUEVO al mismo doctor (B) sí debe dar 409.
	w = doRequest(t, router, http.MethodPost, "/api/staff", tokenB, map[string]any{
		"fullName": "Recepcionista Compartida", "email": "compartida@clinica.test",
	})
	assert.Equal(t, http.StatusConflict, w.Code)

	// El login ahora debe pedir elegir consultorio (2 vínculos activos).
	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login", "", map[string]any{
		"email": "compartida@clinica.test", "password": "claveCompartida1",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	loginResp := decodeJSON(t, w)
	assert.Equal(t, true, loginResp["needsDoctorSelection"])
	selectionToken, _ := loginResp["selectionToken"].(string)
	require.NotEmpty(t, selectionToken)
	doctors, _ := loginResp["doctors"].([]any)
	require.Len(t, doctors, 2)

	// Con el token de selección, elige el consultorio de A y recibe un JWT
	// real, correctamente escopado a docA.
	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login/select-doctor", "", map[string]any{
		"selectionToken": selectionToken, "doctorId": docA.ID,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	selectResp := decodeJSON(t, w)
	assert.Equal(t, "Dra. A", selectResp["doctorName"])
	staffToken, _ := selectResp["token"].(string)
	require.NotEmpty(t, staffToken)

	// Ese token da acceso a los datos del consultorio de A.
	w = doRequest(t, router, http.MethodPost, "/api/patients", staffToken, map[string]any{
		"firstName": "Paciente", "lastName": "DeA", "email": "pacientea_share@test.local",
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	w = doRequest(t, router, http.MethodGet, "/api/patients", tokenA, nil)
	require.Equal(t, http.StatusOK, w.Code)
	patientsA := decodeJSON(t, w)
	dataA, _ := patientsA["data"].([]any)
	assert.Len(t, dataA, 1, "el paciente creado por el personal debe verse en el consultorio de A")

	w = doRequest(t, router, http.MethodGet, "/api/patients", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code)
	patientsB := decodeJSON(t, w)
	dataB, _ := patientsB["data"].([]any)
	assert.Len(t, dataB, 0, "el consultorio de B no debe ver los pacientes creados en el de A")

	// Desactivar el acceso desde el consultorio de B no debe afectar el
	// acceso al consultorio de A.
	w = doRequest(t, router, http.MethodGet, "/api/staff", tokenB, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var staffListB []map[string]any
	decodeJSONList(t, w, &staffListB)
	require.Len(t, staffListB, 1)
	staffIDInB := strconv.Itoa(int(staffListB[0]["id"].(float64)))

	w = doRequest(t, router, http.MethodPut, "/api/staff/"+staffIDInB+"/active", tokenB, map[string]any{
		"active": false,
	})
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login", "", map[string]any{
		"email": "compartida@clinica.test", "password": "claveCompartida1",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	loginAfterDeactivate := decodeJSON(t, w)
	// Ya solo tiene UN vínculo activo (A) — debe volver a dar el token
	// directo, sin pedir selección.
	assert.Nil(t, loginAfterDeactivate["needsDoctorSelection"])
	assert.Equal(t, "Dra. A", loginAfterDeactivate["doctorName"])
}

// TestStaff_SelectDoctor_RejectsInvalidOrForeignSelectionToken cubre los
// casos de rechazo del segundo paso del login: token inventado, y token
// válido pero pidiendo un consultorio al que esa cuenta no tiene acceso.
func TestStaff_SelectDoctor_RejectsInvalidOrForeignSelectionToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	router := server.NewRouter(db)

	docA := testutil.CreateTestDoctor(t, db, "doc_select_a", "password123")
	docOther := testutil.CreateTestDoctor(t, db, "doc_select_other", "password123")
	staff := testutil.CreateTestStaff(t, db, docA.ID, "select_test@consultorio.test", "clave123456")

	w := doRequest(t, router, http.MethodPost, "/api/auth/staff-login/select-doctor", "", map[string]any{
		"selectionToken": "token-inventado-no-existe", "doctorId": docA.ID,
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Un JWT de sesión normal de personal tampoco sirve como token de
	// selección (roles distintos).
	staffToken := testutil.TokenForStaff(t, docA.ID, staff.ID, staff.Email)
	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login/select-doctor", "", map[string]any{
		"selectionToken": staffToken, "doctorId": docA.ID,
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Un token de selección real, pero pidiendo un consultorio al que no
	// tiene ningún vínculo.
	selectionToken, err := auth.GenerateStaffSelectionToken(staff.ID, staff.Email)
	require.NoError(t, err)
	w = doRequest(t, router, http.MethodPost, "/api/auth/staff-login/select-doctor", "", map[string]any{
		"selectionToken": selectionToken, "doctorId": docOther.ID,
	})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	return string(hashed)
}
