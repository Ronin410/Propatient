package database

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"

	_ "embed"

	"propatient-api/internal/models"

	"gorm.io/gorm"
)

// cie10CSV es el catálogo oficial de diagnósticos CIE-10 (DGIS/CENETEC,
// actualización abril 2024), ya filtrado a solo códigos vigentes — ver
// scripts de importación usados una sola vez para generarlo a partir del
// Excel oficial. Va embebido en el binario (go:embed) para que la carga
// inicial no dependa de un archivo aparte en el contenedor ni de una
// llamada de red; es dato de referencia de solo lectura, nunca cambia en
// tiempo de ejecución.
//
//go:embed data/cie10.csv
var cie10CSV []byte

// SeedCie10Catalog carga el catálogo CIE-10 una sola vez (si la tabla
// está vacía) — es un catálogo de referencia, no datos de un consultorio
// en particular, así que no depende de ENABLE_TEST_SEED y sí corre en
// producción. Usa CreateInBatches para no mandar ~12,500 INSERTs
// individuales al arrancar.
func SeedCie10Catalog(db *gorm.DB) {
	var count int64
	db.Model(&models.Cie10Code{}).Count(&count)
	if count > 0 {
		return
	}

	reader := csv.NewReader(bytes.NewReader(cie10CSV))
	rows, err := reader.ReadAll()
	if err != nil {
		log.Printf("⚠️ No se pudo leer el catálogo CIE-10 embebido: %v", err)
		return
	}
	if len(rows) <= 1 {
		return
	}

	codes := make([]models.Cie10Code, 0, len(rows)-1)
	for _, row := range rows[1:] { // rows[0] es el encabezado: code,name,chapter_key,chapter,sex_restriction
		if len(row) < 5 {
			continue
		}
		codes = append(codes, models.Cie10Code{
			Code:           row[0],
			Name:           row[1],
			ChapterKey:     row[2],
			Chapter:        row[3],
			SexRestriction: row[4],
		})
	}

	if err := db.CreateInBatches(codes, 500).Error; err != nil {
		log.Printf("⚠️ No se pudo cargar el catálogo CIE-10: %v", err)
		return
	}
	fmt.Printf("✅ Catálogo CIE-10 cargado: %d códigos vigentes.\n", len(codes))
}
