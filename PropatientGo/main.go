package main

import (
	"log"
	"os"
	"time"

	"propatient-api/internal/database"
	"propatient-api/internal/models"
	"propatient-api/internal/server"
	"propatient-api/internal/workers"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1. Cargar variables de entorno
	//Otro comentairio
	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: No se encontró archivo .env, usando variables de entorno del sistema")
	}

	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("Error crítico: la variable de entorno JWT_SECRET no está definida")
	}

	// 2. Conexión a DB
	dsn := os.Getenv("DATABASE_URL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		log.Fatal("Error crítico al conectar a la DB:", err)
	}

	// 3. Automigración y Seed
	db.AutoMigrate(&models.Doctor{}, &models.Patient{}, &models.MedicalHistory{}, &models.Appointment{}, &models.MedicalDocument{}, &models.DoctorTemplate{})
	database.SeedDatabase(db)

	workers.StartNightClosureWorker(db)

	// 4. Configuración del Router (rutas, CORS, health check en internal/server)
	r := server.NewRouter(db)

	// 5. Lanzar Servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "8095"
	}

	log.Printf("🚀 ProPatient API (Go) corriendo en puerto %s", port)
	r.Run(":" + port)
}
