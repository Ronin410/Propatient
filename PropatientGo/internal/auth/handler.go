package auth

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"propatient-api/internal/models"
	"strconv"
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
			"token": token,
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
			//FullName         string `json:"fullName" binding:"required"`
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
			//FullName:         req.FullName,
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
		// 1. Extraer ID del doctor inyectado por tu Middleware JWT
		doctorID, exists := c.Get("doctorID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Acceso denegado. Sesión no válida."})
			return
		}

		// 2. Extraer campos de texto tradicionales mediante PostForm
		licenseNumber := c.PostForm("licenseNumber")
		rfc := c.PostForm("rfc")
		curp := c.PostForm("curp")
		//feeStr := c.PostForm("baseConsultationFee")

		if licenseNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "El número de cédula es obligatorio"})
			return
		}

		//fee, _ := strconv.ParseFloat(feeStr, 64)

		// 3. Procesar el archivo adjunto de la INE
		file, err := c.FormFile("ineDocument")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "La fotografía de tu identificación (INE) es obligatoria"})
			return
		}

		// Opcional: Guardar el archivo físicamente en un directorio local del servidor
		// Puedes crear una carpeta llamada "uploads" en tu raíz
		filename := "dr_" + strconv.FormatUint(uint64(doctorID.(uint)), 10) + "_" + filepath.Base(file.Filename)
		uploadPath := filepath.Join("uploads", filename)

		if err := c.SaveUploadedFile(file, uploadPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo almacenar el archivo de identidad"})
			return
		}

		// 4. Actualizar el modelo del Doctor en Postgres usando GORM
		err = db.Model(&models.Doctor{}).Where("id = ?", doctorID).Updates(map[string]interface{}{
			"license_number": licenseNumber,
			"rfc":            rfc,
			"curp":           curp,
			//"base_consultation_fee": fee,
			"cedula_validated": "CAPTURADA", // 🚀 Cambiamos el estatus para dejarlo en revisión
		}).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al actualizar la información en base de datos"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Información y documentación recibida de forma exitosa"})
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
