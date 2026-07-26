package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"propatient-api/internal/auth"
	"propatient-api/internal/billing"
	"propatient-api/internal/models"
	"propatient-api/internal/storage"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SuperAdminLoginHandler autentica una cuenta interna de ProPatient (ver
// models.SuperAdmin), totalmente separada de Doctor/Staff. Mismo patrón
// que auth.LoginHandler, pero emite un GenerateSuperAdminToken.
func SuperAdminLoginHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
			return
		}

		var admin models.SuperAdmin
		if err := db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario o contraseña incorrectos"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario o contraseña incorrectos"})
			return
		}

		token, err := auth.GenerateSuperAdminToken(admin.ID, admin.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar la sesión"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": token, "username": admin.Username})
	}
}

// pendingDoctorResponse expone solo lo que el administrador necesita para
// revisar una cédula: nunca el hash de contraseña ni datos clínicos (este
// endpoint no tiene ninguna relación con pacientes).
type pendingDoctorResponse struct {
	ID                uint   `json:"id"`
	FullName          string `json:"fullName"`
	Email             string `json:"email"`
	Username          string `json:"username"`
	LicenseNumber     string `json:"licenseNumber"`
	RFC               string `json:"rfc"`
	CURP              string `json:"curp"`
	University        string `json:"university"`
	IneDocumentURL    string `json:"ineDocumentUrl"`
	CedulaDocumentURL string `json:"cedulaDocumentUrl"`
	CedulaValidated   string `json:"cedulaValidated"`
}

// ListPendingDoctors devuelve los doctores con cédula "CAPTURADA": ya
// terminaron el onboarding y están esperando revisión manual.
func ListPendingDoctors(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var doctors []models.Doctor
		if err := db.Where("cedula_validated = ?", "CAPTURADA").Order("updated_at asc").Find(&doctors).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al consultar doctores pendientes"})
			return
		}

		response := make([]pendingDoctorResponse, 0, len(doctors))
		for _, d := range doctors {
			ineURL := ""
			if d.IneDocumentPath != "" {
				if url, err := storageClient.URL(c.Request.Context(), d.IneDocumentPath); err == nil {
					ineURL = url
				}
			}
			cedulaURL := ""
			if d.CedulaDocumentPath != "" {
				if url, err := storageClient.URL(c.Request.Context(), d.CedulaDocumentPath); err == nil {
					cedulaURL = url
				}
			}
			response = append(response, pendingDoctorResponse{
				ID:                d.ID,
				FullName:          d.FullName,
				Email:             d.Email,
				Username:          d.Username,
				LicenseNumber:     d.LicenseNumber,
				RFC:               d.RFC,
				CURP:              d.CURP,
				University:        d.University,
				IneDocumentURL:    ineURL,
				CedulaDocumentURL: cedulaURL,
				CedulaValidated:   d.CedulaValidated,
			})
		}

		c.JSON(http.StatusOK, response)
	}
}

// ApproveDoctorCedula marca a un doctor como VALIDADA y le avisa por
// correo. Solo tiene efecto si estaba en CAPTURADA — evita "aprobar" a un
// doctor que ni siquiera ha terminado su onboarding.
func ApproveDoctorCedula(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var doctor models.Doctor
		if err := db.First(&doctor, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.CedulaValidated != "CAPTURADA" {
			c.JSON(http.StatusConflict, gin.H{"error": "Este doctor no tiene una cédula pendiente de revisión"})
			return
		}

		if err := db.Model(&doctor).Update("cedula_validated", "VALIDADA").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al aprobar la cédula"})
			return
		}

		// Si a este correo lo habían invitado a una clínica ANTES de que
		// tuviera cuenta (ver handlers.inviteClinicByEmail), esta es la
		// señal de que ya es una cuenta real: se une solo, sin que el
		// dueño tenga que volver a invitarlo ni el doctor tenga que
		// confirmar nada — mejor esfuerzo, un fallo aquí no debe tumbar la
		// aprobación de la cédula en sí.
		joinPendingClinicIfInvited(c.Request.Context(), db, client, &doctor)

		go func(email, name string) {
			defer func() { recover() }()
			if email == "" {
				return
			}
			if err := auth.SendCedulaApprovedEmail(email, name); err != nil {
				fmt.Printf("❌ Fallo al enviar correo de aprobación de cédula a [%s]: %v\n", email, err)
			}
		}(doctor.Email, doctor.FullName)

		c.JSON(http.StatusOK, gin.H{"message": "Cédula aprobada"})
	}
}

// joinPendingClinicIfInvited busca una ClinicEmailInvite sin consumir ni
// vencida para el correo de este doctor y, si existe, lo une a esa
// clínica (mismo camino que aceptar una invitación normal, ver
// joinDoctorToClinic) y marca la invitación como consumida. No hace nada
// si el doctor ya pertenece a alguna clínica (evita pisar una que se
// haya unido por otro medio mientras tanto).
func joinPendingClinicIfInvited(ctx context.Context, db *gorm.DB, client billing.Client, doctor *models.Doctor) {
	if doctor.ClinicID != nil || doctor.Email == "" {
		return
	}

	var invite models.ClinicEmailInvite
	err := db.Where("email = ? AND consumed_at IS NULL AND expires_at > ?", strings.ToLower(doctor.Email), time.Now().UTC()).
		Order("created_at DESC").First(&invite).Error
	if err != nil {
		return // sin invitación pendiente, nada que hacer
	}

	var clinic models.Clinic
	if err := db.First(&clinic, invite.ClinicID).Error; err != nil {
		log.Printf("⚠️ Invitación de clínica por correo apunta a una clínica inexistente (invite %d, clínica %d)", invite.ID, invite.ClinicID)
		return
	}

	if err := joinDoctorToClinic(ctx, db, client, doctor, &clinic); err != nil {
		log.Printf("⚠️ No se pudo unir automáticamente al doctor %d a la clínica %d tras aprobar su cédula: %v", doctor.ID, clinic.ID, err)
		return
	}

	now := time.Now().UTC()
	if err := db.Model(&invite).Update("consumed_at", now).Error; err != nil {
		log.Printf("⚠️ El doctor %d se unió a la clínica %d pero no se pudo marcar la invitación %d como consumida: %v", doctor.ID, clinic.ID, invite.ID, err)
	}
}

// RejectDoctorCedula regresa al doctor a PENDIENTE (puede volver a subir su
// documentación desde ValidateLicense.tsx) y le avisa por correo con un
// motivo opcional.
func RejectDoctorCedula(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req struct {
			Reason string `json:"reason"`
		}
		// El body es opcional: si no viene o no es JSON válido, se rechaza
		// sin motivo en vez de fallar la solicitud completa.
		_ = c.ShouldBindJSON(&req)

		var doctor models.Doctor
		if err := db.First(&doctor, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.CedulaValidated != "CAPTURADA" {
			c.JSON(http.StatusConflict, gin.H{"error": "Este doctor no tiene una cédula pendiente de revisión"})
			return
		}

		if err := db.Model(&doctor).Update("cedula_validated", "PENDIENTE").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al rechazar la cédula"})
			return
		}

		go func(email, name, reason string) {
			defer func() { recover() }()
			if email == "" {
				return
			}
			if err := auth.SendCedulaRejectedEmail(email, name, reason); err != nil {
				fmt.Printf("❌ Fallo al enviar correo de rechazo de cédula a [%s]: %v\n", email, err)
			}
		}(doctor.Email, doctor.FullName, req.Reason)

		c.JSON(http.StatusOK, gin.H{"message": "Cédula rechazada"})
	}
}

type grantFreeAccessRequest struct {
	// Until: fecha (YYYY-MM-DD) hasta la que el doctor tiene acceso sin
	// pagar. Reutiliza el mismo mecanismo que ya usa la prueba gratis de
	// 14 días (SubscriptionStatus "trialing" + TrialEndsAt) — no hace
	// falta un campo ni un estado nuevo, RequireActiveSubscription ya
	// sabe leer esto.
	Until string `json:"until" binding:"required"`
}

// GrantFreeAccess le da acceso manual y gratuito a un doctor hasta una
// fecha fija, decidida por el superadmin — pensado para casos que no
// encajan en el código de invitación automático (ver internal/referral):
// familiares o conocidos cercanos ayudando a difundir la plataforma,
// alguien a quien los 14 días de prueba no le alcanzaron, o un trato
// informal ("tráeme a 3 colegas y te doy 2 meses gratis"). Funciona sobre
// cualquier estado previo — "canceled"/"past_due" (destrabar a alguien que
// ya se había quedado fuera) igual que "trialing" o "active".
//
// Si el doctor ya tenía una suscripción de Stripe activa, se cancela antes
// de marcarlo como "trialing" (mejor esfuerzo) — para no cobrarle Y darle
// acceso gratis al mismo tiempo. No aplica a un doctor de clínica: su
// acceso depende de la suscripción de la clínica completa, no de él solo.
func GrantFreeAccess(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req grantFreeAccessRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Falta la fecha hasta la que quieres dar acceso gratuito."})
			return
		}
		until, err := time.Parse("2006-01-02", req.Until)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La fecha debe tener el formato AAAA-MM-DD."})
			return
		}
		// Fin del día en vez de las 00:00, para que "hasta el 31" incluya
		// todo el 31 y no expire al despertar ese mismo día.
		until = time.Date(until.Year(), until.Month(), until.Day(), 23, 59, 59, 0, time.UTC)
		if !until.After(time.Now().UTC()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La fecha debe ser en el futuro."})
			return
		}

		var doctor models.Doctor
		if err := db.First(&doctor, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.ClinicID != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Este doctor pertenece a una clínica; su acceso depende de la suscripción de la clínica, no se puede dar por separado."})
			return
		}

		if client != nil && doctor.StripeSubscriptionID != "" {
			if err := client.CancelSubscription(c.Request.Context(), doctor.StripeSubscriptionID); err != nil {
				log.Printf("⚠️ No se pudo cancelar la suscripción de Stripe del doctor %d al darle acceso gratuito: %v", doctor.ID, err)
			}
		}

		updates := map[string]interface{}{
			"subscription_status":    "trialing",
			"trial_ends_at":          until,
			"stripe_subscription_id": "",
			"past_due_since":         gorm.Expr("NULL"),
		}
		if err := db.Model(&doctor).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar el acceso del doctor"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":            "Acceso gratuito otorgado",
			"subscriptionStatus": "trialing",
			"trialEndsAt":        until,
		})
	}
}

// GrantClinicFreeAccess es el equivalente de GrantFreeAccess pero para una
// clínica completa: el plan de clínica no tiene periodo de prueba
// automático (el cobro arranca de inmediato al completar el Checkout, ver
// ClinicBaseIncludedDoctors), así que esta es la única forma de darle
// acceso gratuito temporal a una clínica — por ejemplo mientras negocia su
// alta, o como cortesía. Afecta a TODOS los doctores de la clínica a la
// vez (ver RequireActiveSubscription), no a uno solo.
func GrantClinicFreeAccess(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req grantFreeAccessRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Falta la fecha hasta la que quieres dar acceso gratuito."})
			return
		}
		until, err := time.Parse("2006-01-02", req.Until)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La fecha debe tener el formato AAAA-MM-DD."})
			return
		}
		until = time.Date(until.Year(), until.Month(), until.Day(), 23, 59, 59, 0, time.UTC)
		if !until.After(time.Now().UTC()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La fecha debe ser en el futuro."})
			return
		}

		var clinic models.Clinic
		if err := db.First(&clinic, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Clínica no encontrada"})
			return
		}

		if client != nil && clinic.StripeSubscriptionID != "" {
			if err := client.CancelSubscription(c.Request.Context(), clinic.StripeSubscriptionID); err != nil {
				log.Printf("⚠️ No se pudo cancelar la suscripción de Stripe de la clínica %d al darle acceso gratuito: %v", clinic.ID, err)
			}
		}

		updates := map[string]interface{}{
			"subscription_status":    "trialing",
			"trial_ends_at":          until,
			"stripe_subscription_id": "",
			"past_due_since":         gorm.Expr("NULL"),
		}
		if err := db.Model(&clinic).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar el acceso de la clínica"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":            "Acceso gratuito otorgado a la clínica",
			"subscriptionStatus": "trialing",
			"trialEndsAt":        until,
		})
	}
}

// adminMonthlyStats resume, para un doctor o para toda la plataforma, las
// citas cuya fecha cae dentro del mes calendario actual: cuántas en total,
// cuántas de cada desenlace, y cuántas se originaron por cada canal (ver
// models.Appointment.Source). Mismos nombres de estatus que ya usa
// GetConsultorioStats, para consistencia.
type adminMonthlyStats struct {
	Total          int64 `json:"total"`
	Completed      int64 `json:"completed"`
	NoShow         int64 `json:"noShow"`
	Cancelled      int64 `json:"cancelled"`
	BookedByDoctor int64 `json:"bookedByDoctor"`
	BookedPublic   int64 `json:"bookedPublic"`
}

// fetchMonthlyStatsByDoctor calcula, en una sola consulta agregada (no una
// por doctor — importa para no volverse lento en cuanto haya decenas de
// doctores), las estadísticas del mes calendario en curso agrupadas por
// doctor. Un doctor sin ninguna cita este mes simplemente no aparece en el
// mapa devuelto (el llamador debe tratar la ausencia como stats en cero).
func fetchMonthlyStatsByDoctor(db *gorm.DB) (map[uint]adminMonthlyStats, error) {
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	var rows []struct {
		DoctorID uint
		Status   string
		Source   string
		Count    int64
	}
	if err := db.Model(&models.Appointment{}).
		Select("doctor_id, status, source, count(*) as count").
		Where("appointment_date_time >= ? AND appointment_date_time < ?", startOfMonth, endOfMonth).
		Group("doctor_id, status, source").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	stats := make(map[uint]adminMonthlyStats)
	for _, r := range rows {
		s := stats[r.DoctorID]
		s.Total += r.Count
		switch r.Status {
		case "COMPLETED":
			s.Completed += r.Count
		case "NOSHOW":
			s.NoShow += r.Count
		case "CANCELLED":
			s.Cancelled += r.Count
		}
		switch r.Source {
		case "DOCTOR":
			s.BookedByDoctor += r.Count
		case "PUBLIC":
			s.BookedPublic += r.Count
		}
		stats[r.DoctorID] = s
	}
	return stats, nil
}

// adminDoctorResponse expone lo que el panel interno necesita para ver de
// un vistazo la salud de cada cuenta: a diferencia de pendingDoctorResponse
// (solo cédulas por revisar), aquí van TODOS los doctores, con su
// suscripción y su actividad del mes — nunca datos clínicos de pacientes,
// este endpoint no tiene ninguna relación con expedientes.
type adminDoctorResponse struct {
	ID                 uint   `json:"id"`
	FullName           string `json:"fullName"`
	Email              string `json:"email"`
	Username           string `json:"username"`
	CedulaValidated    string `json:"cedulaValidated"`
	SubscriptionStatus string `json:"subscriptionStatus"` // ya resuelto: propio, o el de su clínica si aplica
	// TrialEndsAt cubre tanto la prueba real de 14 días como cualquier
	// acceso gratuito otorgado a mano (ver GrantFreeAccess) — ambos usan
	// el mismo campo, así que esta fecha es "hasta cuándo tiene acceso
	// sin pagar" sin importar cuál de los dos fue.
	TrialEndsAt *time.Time `json:"trialEndsAt"`
	// CurrentPeriodEnd es la fecha de renovación de una suscripción de
	// Stripe YA ACTIVA (pagada de verdad) — consultada en vivo a Stripe,
	// igual que en GetBillingStatus, porque no se guarda en la base de
	// datos. nil si no aplica (no está "active", es de clínica, Stripe no
	// está configurado, o la consulta falló).
	CurrentPeriodEnd *time.Time        `json:"currentPeriodEnd"`
	IsClinicMember   bool              `json:"isClinicMember"`
	IsClinicOwner    bool              `json:"isClinicOwner"`
	ClinicID         *uint             `json:"clinicId,omitempty"`
	ClinicName       string            `json:"clinicName,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	MonthlyStats     adminMonthlyStats `json:"monthlyStats"`
}

// ListAllDoctors devuelve TODOS los doctores registrados (no solo los
// pendientes de cédula, ver ListPendingDoctors) con su estatus de
// suscripción y su actividad de citas del mes en curso.
func ListAllDoctors(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var doctors []models.Doctor
		if err := db.Order("created_at desc").Find(&doctors).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al consultar doctores"})
			return
		}

		var clinics []models.Clinic
		db.Find(&clinics)
		clinicByID := make(map[uint]models.Clinic, len(clinics))
		for _, cl := range clinics {
			clinicByID[cl.ID] = cl
		}

		statsByDoctor, err := fetchMonthlyStatsByDoctor(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al calcular estadísticas"})
			return
		}

		response := make([]adminDoctorResponse, 0, len(doctors))
		for _, d := range doctors {
			subStatus := d.SubscriptionStatus
			isClinicMember := false
			clinicName := ""
			// Para un doctor de clínica, trialEndsAt/currentPeriodEnd deben
			// reflejar la suscripción de LA CLÍNICA (compartida por todos
			// sus doctores), no el campo propio del doctor — que queda
			// obsoleto en cuanto se une a una (ver comentario en
			// Doctor.TrialEndsAt).
			trialEndsAt := d.TrialEndsAt
			stripeSubscriptionID := d.StripeSubscriptionID
			var clinicID *uint
			if d.ClinicID != nil {
				isClinicMember = true
				clinicID = d.ClinicID
				if cl, ok := clinicByID[*d.ClinicID]; ok {
					subStatus = cl.SubscriptionStatus
					clinicName = cl.Name
					trialEndsAt = cl.TrialEndsAt
					stripeSubscriptionID = cl.StripeSubscriptionID
				}
			}

			// Mejor esfuerzo, un doctor a la vez: a este volumen (panel
			// interno, se consulta rara vez) es aceptable; si la lista de
			// doctores activos crece mucho, esto habría que paralelizarlo
			// o dejar de consultarlo en vivo aquí. Para clínica, varios
			// doctores comparten la misma StripeSubscriptionID — se repite
			// la consulta una vez por doctor en vez de cachear por clínica,
			// mismo motivo (volumen bajo, panel interno).
			var currentPeriodEnd *time.Time
			if subStatus == "active" && stripeSubscriptionID != "" && client != nil {
				if end, err := client.GetSubscriptionPeriodEnd(c.Request.Context(), stripeSubscriptionID); err == nil {
					currentPeriodEnd = &end
				} else {
					log.Printf("⚠️ No se pudo consultar la fecha de renovación de Stripe del doctor %d: %v", d.ID, err)
				}
			}

			response = append(response, adminDoctorResponse{
				ID:                 d.ID,
				FullName:           d.FullName,
				Email:              d.Email,
				Username:           d.Username,
				CedulaValidated:    d.CedulaValidated,
				SubscriptionStatus: subStatus,
				TrialEndsAt:        trialEndsAt,
				CurrentPeriodEnd:   currentPeriodEnd,
				IsClinicMember:     isClinicMember,
				IsClinicOwner:      d.IsClinicOwner,
				ClinicID:           clinicID,
				ClinicName:         clinicName,
				CreatedAt:          d.CreatedAt,
				MonthlyStats:       statsByDoctor[d.ID], // cero si no tuvo citas este mes
			})
		}

		c.JSON(http.StatusOK, response)
	}
}

// GetAdminOverview resume la salud de toda la plataforma en un solo
// llamado: cuántos doctores hay y en qué estatus (cédula y suscripción), y
// cuántas citas se han manejado este mes en total. Pensado para las
// tarjetas de resumen arriba de la tabla de ListAllDoctors, no para
// reemplazarla.
func GetAdminOverview(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var totalDoctors int64
		db.Model(&models.Doctor{}).Count(&totalDoctors)

		var cedulaRows []struct {
			CedulaValidated string
			Count           int64
		}
		db.Model(&models.Doctor{}).
			Select("cedula_validated, count(*) as count").
			Group("cedula_validated").
			Scan(&cedulaRows)
		byCedula := make(map[string]int64, len(cedulaRows))
		for _, r := range cedulaRows {
			byCedula[r.CedulaValidated] = r.Count
		}

		// Suscripción: los doctores en clínica no traen un estatus propio
		// relevante (dependen del de su clínica), así que se cuentan aparte
		// en un solo bucket "clinic" en vez de mezclar sus SubscriptionStatus
		// individuales (que suelen quedar en "trialing" desde antes de unirse
		// y ya no significan nada).
		var subRows []struct {
			SubscriptionStatus string
			Count              int64
		}
		db.Model(&models.Doctor{}).
			Where("clinic_id IS NULL").
			Select("subscription_status, count(*) as count").
			Group("subscription_status").
			Scan(&subRows)
		bySub := make(map[string]int64, len(subRows)+1)
		for _, r := range subRows {
			bySub[r.SubscriptionStatus] = r.Count
		}
		var clinicMembers int64
		db.Model(&models.Doctor{}).Where("clinic_id IS NOT NULL").Count(&clinicMembers)
		if clinicMembers > 0 {
			bySub["clinic"] = clinicMembers
		}

		statsByDoctor, err := fetchMonthlyStatsByDoctor(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al calcular estadísticas"})
			return
		}
		var total adminMonthlyStats
		for _, s := range statsByDoctor {
			total.Total += s.Total
			total.Completed += s.Completed
			total.NoShow += s.NoShow
			total.Cancelled += s.Cancelled
			total.BookedByDoctor += s.BookedByDoctor
			total.BookedPublic += s.BookedPublic
		}

		c.JSON(http.StatusOK, gin.H{
			"totalDoctors":          totalDoctors,
			"doctorsByCedulaStatus": byCedula,
			"doctorsBySubscription": bySub,
			"monthlyStats":          total,
		})
	}
}
