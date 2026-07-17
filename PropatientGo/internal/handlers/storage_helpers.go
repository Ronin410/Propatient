package handlers

import (
	"context"

	"propatient-api/internal/models"
	"propatient-api/internal/storage"
)

// presignDoctorFiles reemplaza las referencias guardadas (path local o key
// de S3) por URLs listas para usar antes de mandar la respuesta JSON. Con el
// backend de disco local esto es un no-op (ya son paths públicos); con S3,
// genera una URL firmada temporal por cada campo no vacío.
func presignDoctorFiles(ctx context.Context, storageClient storage.Client, doctor *models.Doctor) {
	if storageClient == nil {
		return
	}
	if doctor.AvatarUrl != "" {
		if url, err := storageClient.URL(ctx, doctor.AvatarUrl); err == nil {
			doctor.AvatarUrl = url
		}
	}
	if doctor.LogoUrl != "" {
		if url, err := storageClient.URL(ctx, doctor.LogoUrl); err == nil {
			doctor.LogoUrl = url
		}
	}
}

// presignAppointmentFiles hace lo mismo para la receta y los documentos
// adjuntos de una cita.
func presignAppointmentFiles(ctx context.Context, storageClient storage.Client, appt *models.Appointment) {
	if storageClient == nil {
		return
	}
	if appt.RecipePDFPath != "" {
		if url, err := storageClient.URL(ctx, appt.RecipePDFPath); err == nil {
			appt.RecipePDFPath = url
		}
	}
	for i := range appt.MedicalDocuments {
		if appt.MedicalDocuments[i].FilePath != "" {
			if url, err := storageClient.URL(ctx, appt.MedicalDocuments[i].FilePath); err == nil {
				appt.MedicalDocuments[i].FilePath = url
			}
		}
	}
}

// presignAppointmentsFiles aplica presignAppointmentFiles a una lista completa.
func presignAppointmentsFiles(ctx context.Context, storageClient storage.Client, appts []models.Appointment) {
	for i := range appts {
		presignAppointmentFiles(ctx, storageClient, &appts[i])
	}
}

// presignGalleryImages reemplaza ImagePath por una URL lista para usar en
// cada foto de la galería del perfil público, mismo criterio que
// presignDoctorFiles.
func presignGalleryImages(ctx context.Context, storageClient storage.Client, images []models.DoctorGalleryImage) {
	if storageClient == nil {
		return
	}
	for i := range images {
		if images[i].ImagePath == "" {
			continue
		}
		if url, err := storageClient.URL(ctx, images[i].ImagePath); err == nil {
			images[i].ImagePath = url
		}
	}
}
