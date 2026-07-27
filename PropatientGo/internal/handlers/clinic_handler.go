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
	"propatient-api/internal/geocoding"
	"propatient-api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const clinicInviteValidity = 72 * time.Hour

// maskEmail oculta la mayor parte del correo para mostrarlo en un mensaje
// de error sin exponerlo por completo (ej. "al**@gmail.com").
func maskEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 1 {
		return email
	}
	visible := 2
	if visible > at {
		visible = at
	}
	return email[:visible] + strings.Repeat("*", at-visible) + email[at:]
}

// clinicInviteWrongAccountMessage distingue una invitación genuinamente
// inválida/consumida de una que sí existe pero le pertenece a OTRA cuenta
// que no es la que tiene la sesión abierta ahora mismo — típicamente
// porque el navegador quedó con la sesión de otro doctor (ver el aviso de
// cambio de sesión entre pestañas en AuthContext.tsx). En ese segundo
// caso el mensaje genérico "invitación no válida" es engañoso, ya que la
// invitación sí es válida, solo hace falta iniciar sesión con la cuenta
// correcta.
func clinicInviteWrongAccountMessage(db *gorm.DB, token string) (string, bool) {
	if token == "" {
		return "", false
	}
	var tokenOwner models.Doctor
	if err := db.Select("id, email").Where("clinic_invite_token = ?", token).First(&tokenOwner).Error; err != nil {
		return "", false
	}
	return fmt.Sprintf("Esta invitación es para la cuenta %s. Cierra sesión e inicia con esa cuenta para aceptarla.", maskEmail(tokenOwner.Email)), true
}

// syncClinicDoctorCount recalcula cuántos doctores tiene la clínica y
// sincroniza el concepto de "doctor extra" en Stripe (se crea/actualiza/
// borra según cruce o no billing.ClinicBaseIncludedDoctors). Se llama
// después de aceptar una invitación o de quitar a un doctor — los dos
// únicos momentos en que el conteo cambia. client puede ser nil (Stripe
// no configurado); en ese caso no hace nada, mismo criterio de mejor
// esfuerzo que el resto de integraciones opcionales.
func syncClinicDoctorCount(ctx context.Context, db *gorm.DB, client billing.Client, clinic *models.Clinic) error {
	if client == nil || clinic.StripeSubscriptionID == "" {
		return nil
	}
	var count int64
	if err := db.Model(&models.Doctor{}).Where("clinic_id = ?", clinic.ID).Count(&count).Error; err != nil {
		return err
	}
	var extra int64
	if count > billing.ClinicBaseIncludedDoctors {
		extra = count - billing.ClinicBaseIncludedDoctors
	}
	newItemID, err := client.SetClinicExtraDoctorQuantity(ctx, clinic.StripeSubscriptionID, clinic.StripeExtraItemID, extra)
	if err != nil {
		return err
	}
	if newItemID != clinic.StripeExtraItemID {
		return db.Model(clinic).Update("stripe_extra_item_id", newItemID).Error
	}
	return nil
}

type createClinicRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateClinic convierte al doctor autenticado en dueño de una clínica
// nueva y arma el Checkout del plan base. A propósito NO toca todavía
// Doctor.ClinicID/IsClinicOwner — eso solo se confirma cuando el webhook
// de Stripe (checkout.session.completed) avisa que el pago se completó
// (ver StripeWebhook). Si el doctor abandona el Checkout sin pagar, la
// fila de Clinic queda huérfana en "incomplete" pero el doctor no pierde
// nada de su cuenta individual — evita el caso feo de dejarlo sin acceso
// por una compra que nunca terminó.
func CreateClinic(db *gorm.DB, client billing.Client, cfg billing.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if client == nil || !cfg.IsClinicConfigured() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "El plan de clínica no está configurado en el servidor."})
			return
		}

		doctorID := c.MustGet("doctorID").(uint)
		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.ClinicID != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Ya perteneces a una clínica"})
			return
		}

		var req createClinicRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		clinic := models.Clinic{Name: req.Name, OwnerDoctorID: doctorID, SubscriptionStatus: "incomplete"}
		if err := db.Create(&clinic).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo crear la clínica"})
			return
		}

		base := frontendRedirectBase()
		url, err := client.CreateClinicCheckoutSession(c.Request.Context(), billing.ClinicCheckoutParams{
			ClinicID:      clinic.ID,
			CustomerEmail: doctor.Email,
			SuccessURL:    base + "/clinica?checkout=success",
			CancelURL:     base + "/clinica?checkout=cancelled",
		})
		if err != nil {
			// El error real de Stripe (ej. "No such price": la llave de
			// precio configurada en STRIPE_CLINIC_BASE_PRICE_ID no existe en
			// el modo — prueba/producción — de la cuenta de Stripe que
			// corresponde a STRIPE_SECRET_KEY) solo se ve en este log; el
			// frontend siempre recibe el mensaje genérico de abajo.
			log.Printf("⚠️ No se pudo crear la sesión de Checkout de la clínica %d (doctor %d): %v", clinic.ID, doctorID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo iniciar el proceso de pago"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}

// CreateClinicPortalSession abre el Customer Portal de Stripe para la
// clínica (cancelar, actualizar tarjeta) — mismo mecanismo que
// CreatePortalSession para la suscripción individual, solo que apunta al
// Customer de la clínica en vez del propio del doctor. Solo el dueño.
func CreateClinicPortalSession(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if client == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "El cobro de suscripción no está configurado en el servidor."})
			return
		}

		doctorID := c.MustGet("doctorID").(uint)
		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.ClinicID == nil || !doctor.IsClinicOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Solo el dueño de la clínica puede gestionar sus pagos"})
			return
		}

		var clinic models.Clinic
		if err := db.First(&clinic, *doctor.ClinicID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Clínica no encontrada"})
			return
		}
		if clinic.StripeCustomerID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Aún no tienes una suscripción con la que abrir el portal de pagos."})
			return
		}

		url, err := client.CreatePortalSession(c.Request.Context(), clinic.StripeCustomerID, frontendRedirectBase()+"/clinica")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo abrir el portal de facturación"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}

type clinicDoctorItem struct {
	ID       uint   `json:"id"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	IsOwner  bool   `json:"isOwner"`
}

// GetClinic devuelve la clínica del doctor autenticado (dueño o miembro),
// su lista de doctores, y un desglose informativo del costo — el cobro
// real siempre lo decide Stripe, esto es solo para que el dueño vea de
// un vistazo qué está pagando.
func GetClinic(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var self models.Doctor
		if err := db.First(&self, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if self.ClinicID == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No perteneces a ninguna clínica"})
			return
		}

		var clinic models.Clinic
		if err := db.First(&clinic, *self.ClinicID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Clínica no encontrada"})
			return
		}

		var doctors []models.Doctor
		if err := db.Where("clinic_id = ?", clinic.ID).Order("is_clinic_owner DESC, full_name ASC").Find(&doctors).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo cargar la lista de doctores"})
			return
		}

		items := make([]clinicDoctorItem, 0, len(doctors))
		for _, d := range doctors {
			items = append(items, clinicDoctorItem{ID: d.ID, FullName: d.FullName, Email: d.Email, IsOwner: d.IsClinicOwner})
		}

		extraDoctors := 0
		if len(doctors) > billing.ClinicBaseIncludedDoctors {
			extraDoctors = len(doctors) - billing.ClinicBaseIncludedDoctors
		}

		// Invitaciones a correos que todavía no tenían cuenta cuando se
		// mandaron (ver inviteClinicByEmail) — visibles mientras no se
		// consuman ni venzan, para que el dueño sepa a quién ya invitó y
		// sigue esperando que se registre y se apruebe.
		var pendingEmailInvites []models.ClinicEmailInvite
		db.Where("clinic_id = ? AND consumed_at IS NULL AND expires_at > ?", clinic.ID, time.Now().UTC()).
			Order("created_at DESC").Find(&pendingEmailInvites)
		emailInviteItems := make([]gin.H, 0, len(pendingEmailInvites))
		for _, inv := range pendingEmailInvites {
			emailInviteItems = append(emailInviteItems, gin.H{
				"email":     inv.Email,
				"invitedAt": inv.CreatedAt,
			})
		}

		// Invitaciones a doctores que YA TENÍAN cuenta cuando se les invitó
		// (ver InviteDoctorToClinic) y todavía no aceptan — antes eran
		// invisibles para el dueño (solo se veían las de correos sin
		// cuenta), lo que hacía parecer que la invitación se había perdido.
		var pendingAccountInvitees []models.Doctor
		db.Select("email, clinic_invite_token_expires_at").
			Where("pending_clinic_id = ? AND clinic_invite_token != '' AND clinic_invite_token_expires_at > ?", clinic.ID, time.Now().UTC()).
			Order("clinic_invite_token_expires_at DESC").Find(&pendingAccountInvitees)
		accountInviteItems := make([]gin.H, 0, len(pendingAccountInvitees))
		for _, inv := range pendingAccountInvitees {
			accountInviteItems = append(accountInviteItems, gin.H{
				"email":     inv.Email,
				"expiresAt": inv.ClinicInviteTokenExpiresAt,
			})
		}

		// pastDueGraceEndsAt: hasta cuándo sigue con acceso pese al cobro
		// fallido (ver billing.PastDuePaymentGraceDuration) — nil si no está
		// en past_due o si nunca se registró cuándo empezó.
		var pastDueGraceEndsAt *time.Time
		if clinic.SubscriptionStatus == "past_due" && clinic.PastDueSince != nil {
			t := clinic.PastDueSince.Add(billing.PastDuePaymentGraceDuration)
			pastDueGraceEndsAt = &t
		}

		c.JSON(http.StatusOK, gin.H{
			"id":                  clinic.ID,
			"name":                clinic.Name,
			"subscriptionStatus":  clinic.SubscriptionStatus,
			"pastDueGraceEndsAt":  pastDueGraceEndsAt,
			"isOwner":             self.IsClinicOwner,
			"doctors":             items,
			"baseIncludedDoctors": billing.ClinicBaseIncludedDoctors,
			"extraDoctors":        extraDoctors,
			// Montos informativos (MXN/mes) — el cobro real vive en Stripe,
			// esto es solo para que el dueño vea el desglose sin tener que
			// entrar al portal de facturación.
			"basePriceDisplay":  3200,
			"extraPriceDisplay": 1000,
			// Ubicación única de la clínica (ver UpdateClinicLocation) — el
			// directorio público la usa para TODOS los doctores de esta
			// clínica en vez de la dirección individual de cada quien.
			"address":               clinic.Address,
			"latitude":              clinic.Latitude,
			"longitude":             clinic.Longitude,
			"pendingEmailInvites":   emailInviteItems,
			"pendingAccountInvites": accountInviteItems,
		})
	}
}

type updateClinicLocationRequest struct {
	Address   string `json:"address"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

// UpdateClinicLocation deja que SOLO el dueño de la clínica fije la
// dirección única que el directorio público va a mostrar para todos sus
// doctores (ver toPublicDoctorSummary en public_handler.go) — un
// consultorio de clínica no es "movible" por cada doctor por separado, a
// diferencia de uno independiente.
func UpdateClinicLocation(db *gorm.DB, geoClient geocoding.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var self models.Doctor
		if err := db.First(&self, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if self.ClinicID == nil || !self.IsClinicOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Solo el dueño de la clínica puede editar su ubicación"})
			return
		}

		var clinic models.Clinic
		if err := db.First(&clinic, *self.ClinicID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Clínica no encontrada"})
			return
		}

		var req updateClinicLocationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
			return
		}

		previousAddress := clinic.Address
		lat, lng, geoErr := geocoding.ResolveCoordinates(
			c.Request.Context(), geoClient, req.Address, previousAddress, clinic.Latitude, clinic.Longitude,
			req.Latitude, req.Longitude,
		)
		if geoErr != nil {
			log.Printf("⚠️ No se pudo geocodificar la dirección de la clínica %d: %v", clinic.ID, geoErr)
		}

		clinic.Address = req.Address
		clinic.Latitude = lat
		clinic.Longitude = lng
		if err := db.Save(&clinic).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar la ubicación de la clínica"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "Ubicación de la clínica actualizada",
			"address":   clinic.Address,
			"latitude":  clinic.Latitude,
			"longitude": clinic.Longitude,
		})
	}
}

type inviteDoctorToClinicRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// clinicEmailInviteValidity: mucho más larga que clinicInviteValidity
// (72h) porque aquí se espera a que alguien vea el correo, decida
// registrarse, complete el onboarding Y su cédula sea aprobada — un
// proceso de días, no de horas.
const clinicEmailInviteValidity = 30 * 24 * time.Hour

// inviteClinicByEmail cubre el caso en que el correo invitado todavía no
// tiene cuenta en ProPatient Clinic: en vez de rechazar la invitación, guarda un
// ClinicEmailInvite y le manda un correo invitándolo a registrarse. En
// cuanto esa cuenta exista y su cédula sea aprobada (ver
// ApproveDoctorCedula), se une sola a la clínica — sin que el dueño
// tenga que volver a invitarlo.
func inviteClinicByEmail(c *gin.Context, db *gorm.DB, owner models.Doctor, email string) {
	// Si ya había una invitación pendiente sin consumir para este mismo
	// correo y clínica, se reutiliza (refresca la fecha) en vez de
	// duplicarla.
	var existing models.ClinicEmailInvite
	found := db.Where("clinic_id = ? AND email = ? AND consumed_at IS NULL", *owner.ClinicID, email).First(&existing).Error == nil

	expires := time.Now().UTC().Add(clinicEmailInviteValidity)
	if found {
		if err := db.Model(&existing).Updates(map[string]interface{}{
			"invited_by_doctor_id": owner.ID,
			"expires_at":           expires,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar la invitación"})
			return
		}
	} else {
		invite := models.ClinicEmailInvite{
			ClinicID:          *owner.ClinicID,
			Email:             email,
			InvitedByDoctorID: owner.ID,
			ExpiresAt:         expires,
		}
		if err := db.Create(&invite).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar la invitación"})
			return
		}
	}

	var clinic models.Clinic
	db.Select("name").First(&clinic, *owner.ClinicID)

	registerURL := frontendRedirectBase() + "/login"
	subject := "Te invitaron a unirte a una clínica en ProPatient Clinic"
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 10px;">
				<h2 style="color: #002d42; text-align: center;">¡Te invitaron a ProPatient Clinic!</h2>
				<p>%s te invitó a unirte a la clínica <strong>%s</strong> en ProPatient Clinic, un sistema de gestión para consultorios médicos. Como todavía no tienes cuenta, primero necesitas registrarte.</p>
				<p style="text-align: center; margin: 28px 0;">
					<a href="%s" style="background-color: #005073; color: #ffffff; padding: 12px 24px; border-radius: 6px; text-decoration: none; font-weight: 600;">Registrarme en ProPatient Clinic</a>
				</p>
				<p>Usa <strong>este mismo correo (%s)</strong> al registrarte. En cuanto tu cuenta esté validada, quedarás agregado a la clínica automáticamente — no necesitas hacer nada más.</p>
				<p style="font-size: 12px; color: #888;">Si no esperabas esta invitación, puedes ignorarla — no se hace ningún cambio a menos que te registres con este correo.</p>
			</div>
		</body>
		</html>
	`, owner.FullName, clinic.Name, registerURL, email)

	if err := auth.SendEmail(email, subject, body); err != nil {
		c.JSON(http.StatusCreated, gin.H{"warning": "La invitación se guardó pero no se pudo enviar el correo."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Ese correo todavía no tiene cuenta en ProPatient Clinic — le enviamos una invitación para que se registre. En cuanto su cuenta quede aprobada, se une a la clínica automáticamente."})
}

// InviteDoctorToClinic manda una invitación a un doctor — si ya tiene
// cuenta en ProPatient Clinic, confirma con un clic (AcceptClinicInvite); si el
// correo todavía no tiene cuenta, ver inviteClinicByEmail. Ruta
// protegida, solo el dueño de la clínica puede invitar.
func InviteDoctorToClinic(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ownerID := c.MustGet("doctorID").(uint)

		var owner models.Doctor
		if err := db.First(&owner, ownerID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if owner.ClinicID == nil || !owner.IsClinicOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Solo el dueño de la clínica puede invitar doctores"})
			return
		}

		var req inviteDoctorToClinicRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		email := strings.ToLower(strings.TrimSpace(req.Email))

		var invitee models.Doctor
		if err := db.Where("LOWER(email) = ?", email).First(&invitee).Error; err != nil {
			inviteClinicByEmail(c, db, owner, email)
			return
		}
		if invitee.ID == ownerID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No puedes invitarte a ti mismo"})
			return
		}
		if invitee.ClinicID != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Ese doctor ya pertenece a una clínica"})
			return
		}

		token, err := generateInviteToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo generar la invitación"})
			return
		}
		expires := time.Now().UTC().Add(clinicInviteValidity)

		err = db.Model(&invitee).Updates(map[string]interface{}{
			"pending_clinic_id":              *owner.ClinicID,
			"clinic_invite_token":            token,
			"clinic_invite_token_expires_at": expires,
		}).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar la invitación"})
			return
		}

		var clinic models.Clinic
		db.Select("name").First(&clinic, *owner.ClinicID)

		inviteURL := fmt.Sprintf("%s/clinica/invitacion/%s", frontendRedirectBase(), token)
		subject := "Te invitaron a unirte a una clínica en ProPatient Clinic"
		body := fmt.Sprintf(`
			<html>
			<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
				<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 10px;">
					<h2 style="color: #002d42; text-align: center;">¡Hola, %s!</h2>
					<p>%s te invitó a unirte a la clínica <strong>%s</strong> en ProPatient Clinic. Al aceptar, tu suscripción individual (si tenías una) se cancela y quedas cubierto por el plan de la clínica.</p>
					<p style="text-align: center; margin: 28px 0;">
						<a href="%s" style="background-color: #005073; color: #ffffff; padding: 12px 24px; border-radius: 6px; text-decoration: none; font-weight: 600;">Ver invitación</a>
					</p>
					<p style="font-size: 12px; color: #888;">Este link vence en 72 horas. Si no esperabas esta invitación, puedes ignorarla — no se hace ningún cambio hasta que la aceptes.</p>
				</div>
			</body>
			</html>
		`, invitee.FullName, owner.FullName, clinic.Name, inviteURL)

		if err := auth.SendEmail(email, subject, body); err != nil {
			c.JSON(http.StatusCreated, gin.H{"warning": "La invitación se guardó pero no se pudo enviar el correo."})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Invitación enviada"})
	}
}

// GetClinicInvite muestra los datos de una invitación pendiente (nombre de
// la clínica y de quien invitó) sin aceptarla todavía — así el frontend
// puede mostrar "¿Quieres unirte a la clínica X?" antes de que el doctor
// confirme con AcceptClinicInvite. Protegida (el token por sí solo no
// alcanza, tiene que coincidir con el doctor autenticado) porque a
// diferencia de una invitación de personal, aquí el invitado ya tiene
// cuenta propia — no hace falta un flujo público sin sesión.
func GetClinicInvite(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		token := c.Param("token")

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.ClinicInviteToken == "" || doctor.ClinicInviteToken != token || doctor.PendingClinicID == nil {
			if msg, isWrongAccount := clinicInviteWrongAccountMessage(db, token); isWrongAccount {
				c.JSON(http.StatusForbidden, gin.H{"error": msg, "wrongAccount": true})
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "Invitación no válida"})
			return
		}
		if doctor.ClinicInviteTokenExpiresAt == nil || doctor.ClinicInviteTokenExpiresAt.Before(time.Now().UTC()) {
			c.JSON(http.StatusGone, gin.H{"error": "Esta invitación ya venció. Pide que te inviten de nuevo."})
			return
		}
		if doctor.ClinicID != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Ya perteneces a una clínica"})
			return
		}

		var clinic models.Clinic
		if err := db.First(&clinic, *doctor.PendingClinicID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "La clínica ya no existe"})
			return
		}
		var owner models.Doctor
		db.Select("full_name").First(&owner, clinic.OwnerDoctorID)

		c.JSON(http.StatusOK, gin.H{
			"clinicName": clinic.Name,
			"ownerName":  owner.FullName,
		})
	}
}

// joinDoctorToClinic hace el trabajo real de meter a un doctor a una
// clínica — compartido entre AcceptClinicInvite (el doctor ya tenía
// cuenta y confirma con un clic) y el enganche automático de
// ApproveDoctorCedula (el doctor no tenía cuenta cuando lo invitaron por
// correo, ver ClinicEmailInvite; aquí no hay paso de confirmación
// separado, la aprobación de su cédula ya es la señal de que es una
// cuenta real). Cancela su suscripción individual si tenía una activa
// (para que no le sigan cobrando dos veces) y ajusta el concepto de
// "doctor extra" de la clínica si hace falta. Mejor esfuerzo en la parte
// de Stripe — un fallo ahí no debe impedir que el doctor quede
// vinculado.
func joinDoctorToClinic(ctx context.Context, db *gorm.DB, client billing.Client, doctor *models.Doctor, clinic *models.Clinic) error {
	if client != nil && doctor.StripeSubscriptionID != "" {
		if err := client.CancelSubscription(ctx, doctor.StripeSubscriptionID); err != nil {
			fmt.Printf("⚠️ No se pudo cancelar la suscripción individual del doctor %d al unirse a la clínica %d: %v\n", doctor.ID, clinic.ID, err)
		}
	}

	updates := map[string]interface{}{
		"clinic_id":                      clinic.ID,
		"is_clinic_owner":                false,
		"pending_clinic_id":              nil,
		"clinic_invite_token":            "",
		"clinic_invite_token_expires_at": nil,
		"subscription_status":            "active", // cubierto por la clínica, ya no depende de su propio trial/pago
		"stripe_subscription_id":         "",
	}
	if err := db.Model(doctor).Updates(updates).Error; err != nil {
		return err
	}

	if err := syncClinicDoctorCount(ctx, db, client, clinic); err != nil {
		fmt.Printf("⚠️ No se pudo sincronizar el conteo de doctores de la clínica %d con Stripe: %v\n", clinic.ID, err)
	}
	return nil
}

// AcceptClinicInvite confirma que el doctor autenticado (el propio
// invitado, con su sesión normal) acepta unirse a la clínica que lo
// invitó.
func AcceptClinicInvite(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		token := c.Param("token")

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.ClinicInviteToken == "" || doctor.ClinicInviteToken != token || doctor.PendingClinicID == nil {
			if msg, isWrongAccount := clinicInviteWrongAccountMessage(db, token); isWrongAccount {
				c.JSON(http.StatusForbidden, gin.H{"error": msg, "wrongAccount": true})
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "Invitación no válida"})
			return
		}
		if doctor.ClinicInviteTokenExpiresAt == nil || doctor.ClinicInviteTokenExpiresAt.Before(time.Now().UTC()) {
			c.JSON(http.StatusGone, gin.H{"error": "Esta invitación ya venció. Pide que te inviten de nuevo."})
			return
		}
		if doctor.ClinicID != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Ya perteneces a una clínica"})
			return
		}

		var clinic models.Clinic
		if err := db.First(&clinic, *doctor.PendingClinicID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "La clínica ya no existe"})
			return
		}

		if err := joinDoctorToClinic(c.Request.Context(), db, client, &doctor, &clinic); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo completar la unión a la clínica"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Te uniste a la clínica", "clinicName": clinic.Name})
	}
}

// RemoveDoctorFromClinic saca a un doctor de la clínica (el dueño no
// puede quitarse a sí mismo por aquí — cancelar la clínica completa se
// hace desde el portal de Stripe). Ajusta el concepto de "doctor extra"
// si el conteo baja de vuelta a billing.ClinicBaseIncludedDoctors o menos.
func RemoveDoctorFromClinic(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ownerID := c.MustGet("doctorID").(uint)
		targetID := c.Param("id")

		var owner models.Doctor
		if err := db.First(&owner, ownerID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if owner.ClinicID == nil || !owner.IsClinicOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Solo el dueño de la clínica puede quitar doctores"})
			return
		}

		var target models.Doctor
		if err := db.Where("id = ? AND clinic_id = ?", targetID, *owner.ClinicID).First(&target).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ese doctor no pertenece a tu clínica"})
			return
		}
		if target.IsClinicOwner {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El dueño no puede quitarse a sí mismo — cancela la clínica desde el portal de facturación"})
			return
		}

		if err := db.Model(&target).Updates(map[string]interface{}{
			"clinic_id":           nil,
			"subscription_status": "canceled",
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo quitar al doctor de la clínica"})
			return
		}

		var clinic models.Clinic
		if err := db.First(&clinic, *owner.ClinicID).Error; err == nil {
			if err := syncClinicDoctorCount(c.Request.Context(), db, client, &clinic); err != nil {
				fmt.Printf("⚠️ No se pudo sincronizar el conteo de doctores de la clínica %d con Stripe: %v\n", clinic.ID, err)
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Doctor removido de la clínica"})
	}
}

// LeaveClinic deja que un doctor (no dueño) se salga de su clínica por su
// propia cuenta, sin depender de que el dueño lo quite desde
// RemoveDoctorFromClinic — mismo efecto (clinic_id se limpia, la
// suscripción queda "canceled" hasta que se suscriba por su cuenta o lo
// vuelvan a invitar). El dueño no puede salirse por aquí, mismo criterio
// que RemoveDoctorFromClinic: tiene que cancelar la clínica completa desde
// el portal de Stripe, porque sin dueño la clínica se queda huérfana.
func LeaveClinic(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if doctor.ClinicID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No perteneces a ninguna clínica"})
			return
		}
		if doctor.IsClinicOwner {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El dueño no puede salirse de su propia clínica — usa \"Eliminar clínica\" en Mi Clínica para disolverla por completo"})
			return
		}

		clinicID := *doctor.ClinicID
		if err := db.Model(&doctor).Updates(map[string]interface{}{
			"clinic_id":           nil,
			"subscription_status": "canceled",
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo salir de la clínica"})
			return
		}

		var clinic models.Clinic
		if err := db.First(&clinic, clinicID).Error; err == nil {
			if err := syncClinicDoctorCount(c.Request.Context(), db, client, &clinic); err != nil {
				fmt.Printf("⚠️ No se pudo sincronizar el conteo de doctores de la clínica %d con Stripe tras que el doctor %d saliera: %v\n", clinic.ID, doctorID, err)
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Saliste de la clínica"})
	}
}

// DeleteClinic disuelve la clínica por completo — solo el dueño. A
// diferencia de cancelar el pago desde el portal de Stripe (que solo
// marca Clinic.SubscriptionStatus como "canceled" pero deja a todos los
// doctores con su ClinicID apuntando a una clínica ya muerta, sin ninguna
// forma de salir de ahí — ver StripeWebhook, caso
// customer.subscription.deleted), esto: cancela la suscripción de Stripe
// si seguía viva, libera a TODOS los doctores (dueño incluido, para que
// pueda volver a ser un doctor individual o crear otra clínica) y borra
// la fila de la clínica.
func DeleteClinic(db *gorm.DB, client billing.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ownerID := c.MustGet("doctorID").(uint)

		var owner models.Doctor
		if err := db.First(&owner, ownerID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}
		if owner.ClinicID == nil || !owner.IsClinicOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Solo el dueño de la clínica puede eliminarla"})
			return
		}

		var clinic models.Clinic
		if err := db.First(&clinic, *owner.ClinicID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "La clínica ya no existe"})
			return
		}

		// Mejor esfuerzo: si Stripe no responde, se sigue disolviendo la
		// clínica igual — no tiene caso dejar a todos bloqueados por un
		// problema al cancelar un cobro que de cualquier forma ya no
		// tendrá ninguna clínica detrás para justificarlo.
		if client != nil && clinic.StripeSubscriptionID != "" {
			if err := client.CancelSubscription(c.Request.Context(), clinic.StripeSubscriptionID); err != nil {
				log.Printf("⚠️ No se pudo cancelar la suscripción de Stripe al eliminar la clínica %d: %v", clinic.ID, err)
			}
		}

		if err := db.Model(&models.Doctor{}).Where("clinic_id = ?", clinic.ID).Updates(map[string]interface{}{
			"clinic_id":           nil,
			"is_clinic_owner":     false,
			"subscription_status": "canceled",
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo liberar a los doctores de la clínica"})
			return
		}

		if err := db.Delete(&clinic).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo eliminar la clínica"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Clínica eliminada"})
	}
}
