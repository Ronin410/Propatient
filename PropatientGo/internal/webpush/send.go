package webpush

import (
	"context"
	"log"

	"propatient-api/internal/models"

	"gorm.io/gorm"
)

// SendToDoctor manda payload a cada dispositivo/navegador suscrito de un
// doctor (puede tener varios — celular, tablet, distintos navegadores) y
// borra las suscripciones que el proveedor push ya rechazó por expiradas
// (ErrSubscriptionExpired) para que no se sigan acumulando filas muertas.
// Mejor esfuerzo: un fallo en un dispositivo no detiene el envío a los
// demás. client puede ser nil (VAPID no configurado), en cuyo caso no hace
// nada, mismo criterio que whatsapp.SendWithFallback con client == nil.
func SendToDoctor(ctx context.Context, db *gorm.DB, client Client, doctorID uint, payload []byte) {
	if client == nil {
		return
	}

	var subs []models.PushSubscription
	if err := db.Where("doctor_id = ?", doctorID).Find(&subs).Error; err != nil {
		log.Printf("⚠️ No se pudieron consultar las suscripciones push del doctor %d: %v", doctorID, err)
		return
	}

	for _, sub := range subs {
		err := client.SendNotification(ctx, Subscription{
			Endpoint:  sub.Endpoint,
			P256dhKey: sub.P256dhKey,
			AuthKey:   sub.AuthKey,
		}, payload)
		if err == nil {
			continue
		}

		if err == ErrSubscriptionExpired {
			if delErr := db.Delete(&models.PushSubscription{}, sub.ID).Error; delErr != nil {
				log.Printf("⚠️ No se pudo borrar la suscripción push expirada %d: %v", sub.ID, delErr)
			}
			continue
		}

		log.Printf("⚠️ No se pudo enviar la notificación push al doctor %d (suscripción %d): %v", doctorID, sub.ID, err)
	}
}
