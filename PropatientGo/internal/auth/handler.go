package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"propatient-api/internal/models"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

const (
	SMTPServer = "smtp.gmail.com"
	SMTPPort   = "587"
)

func GoogleLoginHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			IDToken string `json:"idToken" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Token de Google requerido"})
			return
		}

		// 1. Validar el ID Token directamente con la API pública de Google
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + req.IDToken)
		if err != nil || resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "El token de Google no es válido o expiró"})
			return
		}
		defer resp.Body.Close()

		var claims models.GoogleTokenClaims
		if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al procesar la identidad de Google"})
			return
		}

		if claims.EmailVerified != "true" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "La cuenta de Google no está verificada"})
			return
		}

		var doctor models.Doctor
		// 2. Buscar si el médico ya existe por su Email institucional/personal
		err = db.Where("email = ?", claims.Email).First(&doctor).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// 3. Si NO existe, creamos un nuevo registro base con banderas en false
				// Usamos la primera parte del correo como username único por defecto
				generatedUsername := strings.Split(claims.Email, "@")[0]

				doctor = models.Doctor{
					Username:         generatedUsername,
					Email:            claims.Email,
					FullName:         claims.Name, // Nombre por defecto de Google
					ProfileCompleted: false,
					CedulaValidated:  "VACIO",
				}

				if err := db.Create(&doctor).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo registrar el nuevo usuario médico"})
					return
				}
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error en la base de datos"})
				return
			}
		}

		// 4. Generar el JWT nativo de tu sistema (reutilizando tu función interna)
		token, err := GenerateToken(doctor.ID, doctor.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar la sesión interna"})
			return
		}

		// 5. Responder con la estructura que espera tu Login.tsx modificado
		c.JSON(http.StatusOK, gin.H{
			"token":    token,
			"fullName": doctor.FullName,
			"userStatus": gin.H{
				"perfilCompletado": doctor.ProfileCompleted,
				"cedulaValidada":   doctor.CedulaValidated,
			},
		})
	}
}

// Handler para la segunda pantalla: Carga de datos extras del Perfil
func UpdateProfileHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// NOTA: Aquí debes extraer el ID del doctor desde el JWT que envía el Middleware de autenticación
		doctorID, _ := c.Get("doctorID")

		var req struct {
			FullName         string `json:"fullName" binding:"required"`
			MedicalSpecialty string `json:"medicalSpecialty" binding:"required"`
			Phone            string `json:"phone" binding:"required"`
			BirthDate        string `json:"birthDate"`
			Address          string `json:"address"`
			PostalCode       string `json:"postalCode"`
			//RFC              string `json:"rfc"`
			//CURP             string `json:"curp"`
			University string `json:"university"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Información de perfil incompleta"})
			return
		}

		parsedDate, err := time.Parse("2006-01-02", req.BirthDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Formato de fecha de nacimiento inválido"})
			return
		}

		// Actualizamos los datos del doctor y marcamos perfil como completado
		err = db.Model(&models.Doctor{}).Where("id = ?", doctorID).Updates(models.Doctor{
			FullName:         req.FullName,
			MedicalSpecialty: req.MedicalSpecialty,
			Phone:            req.Phone,
			ProfileCompleted: true,
			BirthDate:        &parsedDate, // <--- Nota el '&' para que coincida con el puntero *time.Time
			Address:          req.Address,
			PostalCode:       req.PostalCode,
			//RFC:              req.RFC,
			//CURP:             req.CURP,
			University: req.University,
		}).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo actualizar el perfil"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Perfil actualizado exitosamente"})
	}
}

func UpdateLicenseFullHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. OBTENER AL DOCTOR LOGUEADO DESDE EL CONTEXTO JWT DE FORMA SEGURA
		doctorContext, exists := c.Get("doctorID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No autorizado. Inicie sesión de nuevo."})
			return
		}

		// Convertir a uint tolerando cualquier tipo de dato numérico del claim JWT (int, float64, etc.)
		var doctorID uint
		switch v := doctorContext.(type) {
		case uint:
			doctorID = v
		case int:
			doctorID = uint(v)
		case float64:
			doctorID = uint(v)
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Formato de ID de usuario inválido en el servidor"})
			return
		}

		// 2. LEER LOS CAMPOS DEL FORMULARIO MULTIPART (Enviados desde el FormData de React)
		licenseNumber := strings.TrimSpace(c.PostForm("licenseNumber"))
		rfc := strings.TrimSpace(strings.ToUpper(c.PostForm("rfc")))
		curp := strings.TrimSpace(strings.ToUpper(c.PostForm("curp")))

		if licenseNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El número de cédula profesional es un campo obligatorio."})
			return
		}

		// 3. RECUPERAR EL ARCHIVO FÍSICO DE LA IDENTIFICACIÓN
		file, err := c.FormFile("ineDocument")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La foto o documento de tu identificación oficial (INE/Pasaporte) es requerido."})
			return
		}

		// 4. CREAR NOMBRE ÚNICO E INMUTABLE PARA EL ARCHIVO LIGADO AL DOCTOR
		ext := filepath.Ext(file.Filename)
		uniqueFileName := fmt.Sprintf("ine_doctor_%d_%d%s", doctorID, time.Now().Unix(), ext)

		// Carpeta destino (Asegúrate de que este directorio exista en tu servidor o créalo dinámicamente)
		uploadFolder := "./uploads/documentos_identidad/"
		dst := filepath.Join(uploadFolder, uniqueFileName)

		// Guardar el archivo físicamente en el disco
		if err := c.SaveUploadedFile(file, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo almacenar el archivo de identidad en el servidor."})
			return
		}

		// 5. SELECCIONAR Y ACTUALIZAR EL REGISTRO EN LA BASE DE DATOS
		var doctor models.Doctor
		if err := db.First(&doctor, doctorID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Doctor no encontrado"})
			return
		}

		// Mapear los campos al registro recuperado
		doctor.LicenseNumber = licenseNumber
		doctor.RFC = rfc
		doctor.CURP = curp
		doctor.IneDocumentPath = dst         // Guardamos la ruta relativa del archivo guardado
		doctor.CedulaValidated = "CAPTURADA" // Cambia el estado para que el Login detecte el mensaje de espera

		// Persistir cambios con GORM
		if err := db.Save(&doctor).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar los datos profesionales en la base de datos."})
			return
		}

		// 🚀 6. DISPARAR ENVÍO DE CORREO EN SEGUNDO PLANO (ASÍNCRONO)
		// Usamos una Goroutine para que el API responda de inmediato y no se quede colgada esperando al servidor SMTP
		go func(email string, name string) {
			if email != "" {
				err := SendValidationEmail(email, name)
				if err != nil {
					fmt.Printf("❌ Fallo crítico al enviar correo de validación a [%s]: %v\n", email, err)
				} else {
					fmt.Printf("📩 Correo de validación enviado exitosamente a [%s]\n", email)
				}
			}
		}(doctor.Email, doctor.FullName)

		// 7. RESPUESTA EXITOSA HACIA TU COMPONENTE REACT
		c.JSON(http.StatusOK, gin.H{
			"message": "Información y documentación adjuntada con éxito.",
			"status":  doctor.CedulaValidated,
		})
	}
}

func UpdateLicenseHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID, exists := c.Get("doctorID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Acceso no autorizado"})
			return
		}

		var req struct {
			LicenseNumber       string  `json:"licenseNumber" binding:"required"`
			RFC                 string  `json:"rfc"`
			CURP                string  `json:"curp"`
			BaseConsultationFee float64 `json:"baseConsultationFee"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El número de cédula profesional es obligatorio"})
			return
		}

		err := db.Model(&models.Doctor{}).Where("id = ?", doctorID).Updates(map[string]interface{}{
			"license_number": req.LicenseNumber,
			"rfc":            req.RFC,
			"curp":           req.CURP,
			//"base_consultation_fee": req.BaseConsultationFee,
			"cedula_validated": "CAPTURADA", // Termina el onboarding completo
		}).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al guardar los datos de validación"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Validación profesional exitosa"})
	}
}

func LoginHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest

		// 1. Validar formato del JSON de entrada
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Datos inválidos"})
			return
		}

		var doctor models.Doctor
		// 2. Buscar doctor por username en la base de datos
		if err := db.Where("username = ?", req.Username).First(&doctor).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario o contraseña incorrectos"})
			return
		}

		// 3. Verificar Password usando Bcrypt
		err := bcrypt.CompareHashAndPassword([]byte(doctor.PasswordHash), []byte(req.Password))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Usuario o contraseña incorrectos"})
			return
		}

		// 4. Generar Token (PASANDO EL ID Y EL USERNAME)
		// IMPORTANTE: doctor.ID es el que usaremos en las relaciones de la DB
		token, err := GenerateToken(doctor.ID, doctor.Username)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar la sesión"})
			return
		}

		// 5. Respuesta exitosa
		c.JSON(http.StatusOK, gin.H{
			"token":    token,
			"fullName": doctor.FullName,
			"username": doctor.Username,
		})
	}
}

func RegisterDoctor(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username  string `json:"username" binding:"required"`
			Password  string `json:"password" binding:"required"`
			FullName  string `json:"full_name" binding:"required"`
			Specialty string `json:"specialty"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Datos incompletos"})
			return
		}

		// Encriptar contraseña
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

		doctor := models.Doctor{
			Username:         req.Username,
			PasswordHash:     string(hashedPassword),
			FullName:         req.FullName,
			MedicalSpecialty: req.Specialty,
		}

		if err := db.Create(&doctor).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "El nombre de usuario ya está en uso"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Doctor registrado exitosamente"})
	}
}

func SendValidationEmail(toEmail string, doctorName string) error {

	senderEmail := os.Getenv("SMTP_EMAIL")
	senderPass := os.Getenv("SMTP_PASSWORD")

	auth := smtp.PlainAuth("", senderEmail, senderPass, SMTPServer)

	subject := "Subject: ProPatient - Tu cuenta está en proceso de validación\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"

	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 10px;">
				<h2 style="color: #6a11cb; text-align: center;">¡Hola, Dr(a). %s!</h2>
				<p>Hemos recibido correctamente tu documentación e información profesional (Cédula, CURP, RFC e Identificación Oficial).</p>
				
				<div style="background-color: #f8f9fa; padding: 15px; border-left: 4px solid #aa3bff; margin: 20px 0;">
					<p style="margin: 0; font-weight: bold;">Estado actual de tu cuenta: <span style="color: #d97706;">PENDIENTE DE VALIDACIÓN</span></p>
				</div>

				<p>Nuestro equipo técnico está verificando la autenticidad de tu cédula profesional ante el Registro Nacional de Profesionistas. Este proceso toma habitualmente entre 12 y 24 horas hábiles.</p>
				<p>Una vez aprobada tu identidad médica, recibirás un correo de confirmación y podrás acceder de forma completa a todas las herramientas de <strong>ProPatient Medical System</strong>.</p>
				
				<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;" />
				<p style="font-size: 12px; color: #888; text-align: center;">Este es un correo automático, por favor no respondas a este mensaje.</p>
			</div>
		</body>
		</html>
	`, doctorName)

	msg := []byte(subject + mime + body)
	addr := SMTPServer + ":" + SMTPPort

	return smtp.SendMail(addr, auth, senderEmail, []string{toEmail}, msg)
}
