package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"propatient-api/internal/models"
	"propatient-api/internal/storage"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// maxGalleryImages limita cuántas fotos adicionales puede tener un doctor
// en su perfil público — sin límite, un doctor podría llenar el bucket/
// disco sin querer subiendo la misma sesión de fotos varias veces.
const maxGalleryImages = 8

// AddGalleryImage sube una foto nueva a la galería del perfil público del
// doctor autenticado (solo doctor, ver router.go). Rechaza con 400 si ya
// llegó al límite de maxGalleryImages, para que el doctor tenga que borrar
// alguna antes de seguir subiendo.
func AddGalleryImage(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var count int64
		db.Model(&models.DoctorGalleryImage{}).Where("doctor_id = ?", doctorID).Count(&count)
		if count >= maxGalleryImages {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Ya tienes el máximo de %d fotos en tu galería. Elimina alguna antes de subir otra.", maxGalleryImages)})
			return
		}

		file, err := c.FormFile("image")
		if err != nil || file == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Falta la imagen a subir"})
			return
		}
		if err := storage.ValidateUploadedFile(file, storage.UploadKindAvatarOrLogo); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ext := filepath.Ext(file.Filename)
		key := fmt.Sprintf("profiles/doc_%d_gallery_%d%s", doctorID, time.Now().UnixNano(), ext)
		storedRef, err := storageClient.Save(c.Request.Context(), key, file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar la imagen"})
			return
		}

		image := models.DoctorGalleryImage{DoctorID: doctorID, ImagePath: storedRef}
		if err := db.Create(&image).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo guardar la imagen"})
			return
		}

		if url, err := storageClient.URL(c.Request.Context(), image.ImagePath); err == nil {
			image.ImagePath = url
		}
		c.JSON(http.StatusCreated, image)
	}
}

// ListGalleryImages devuelve todas las fotos del doctor autenticado, con
// URLs ya listas para usar.
func ListGalleryImages(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)

		var images []models.DoctorGalleryImage
		if err := db.Where("doctor_id = ?", doctorID).Order("created_at ASC").Find(&images).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudieron obtener las fotos"})
			return
		}

		presignGalleryImages(c.Request.Context(), storageClient, images)
		c.JSON(http.StatusOK, images)
	}
}

// DeleteGalleryImage borra una foto del doctor autenticado, del storage y
// de la base de datos. 404 si el ID no existe o no es de este doctor (evita
// que un doctor borre la foto de otro adivinando el ID).
func DeleteGalleryImage(db *gorm.DB, storageClient storage.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		doctorID := c.MustGet("doctorID").(uint)
		id := c.Param("id")

		var image models.DoctorGalleryImage
		if err := db.Where("id = ? AND doctor_id = ?", id, doctorID).First(&image).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Foto no encontrada"})
			return
		}

		if err := storageClient.Delete(c.Request.Context(), image.ImagePath); err != nil {
			// No bloqueamos el borrado del registro por un error al borrar
			// el archivo físico (ej. ya no existía) — mejor un registro
			// huérfano ocasional en el storage que dejar la foto "atorada"
			// en la galería sin poder quitarla.
			c.Error(err)
		}

		if err := db.Delete(&image).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No se pudo eliminar la foto"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Foto eliminada"})
	}
}
