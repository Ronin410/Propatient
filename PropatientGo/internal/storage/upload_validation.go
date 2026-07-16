package storage

import (
	"fmt"
	"mime/multipart"
	"net/http"
)

// UploadKind agrupa el límite de tamaño y los tipos de contenido permitidos
// para una categoría de archivo subido (avatar, documento clínico, etc.).
// Vive en este paquete (no en internal/handlers) para que tanto
// internal/handlers como internal/auth puedan usarlo sin crear un import
// cíclico entre ambos.
type UploadKind struct {
	label               string
	maxSizeBytes        int64
	allowedContentTypes map[string]bool
}

var imageContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

var (
	UploadKindAvatarOrLogo = UploadKind{
		label:               "la imagen",
		maxSizeBytes:        3 << 20, // 3 MiB
		allowedContentTypes: imageContentTypes,
	}
	UploadKindMedicalDocument = UploadKind{
		label:        "el documento",
		maxSizeBytes: 15 << 20, // 15 MiB
		allowedContentTypes: mergeContentTypes(imageContentTypes, map[string]bool{
			"application/pdf": true,
		}),
	}
	UploadKindIneDocument = UploadKind{
		label:        "la identificación oficial",
		maxSizeBytes: 10 << 20, // 10 MiB
		allowedContentTypes: mergeContentTypes(imageContentTypes, map[string]bool{
			"application/pdf": true,
		}),
	}
	UploadKindRecipePDF = UploadKind{
		label:        "la receta",
		maxSizeBytes: 5 << 20, // 5 MiB
		allowedContentTypes: map[string]bool{
			"application/pdf": true,
		},
	}
)

func mergeContentTypes(sets ...map[string]bool) map[string]bool {
	merged := make(map[string]bool)
	for _, set := range sets {
		for k, v := range set {
			merged[k] = v
		}
	}
	return merged
}

// ValidateUploadedFile rechaza archivos demasiado grandes o de un tipo no
// permitido, ANTES de guardarlos (disco local o S3, ver Client.Save). El
// tipo se detecta leyendo los primeros bytes reales del archivo
// (http.DetectContentType), no el header "Content-Type" que manda el
// navegador — ese lo controla quien sube el archivo y es trivial de
// falsificar.
func ValidateUploadedFile(file *multipart.FileHeader, kind UploadKind) error {
	if file.Size > kind.maxSizeBytes {
		return fmt.Errorf("%s no debe superar los %d MB (recibido: %.1f MB)", kind.label, kind.maxSizeBytes/(1<<20), float64(file.Size)/(1<<20))
	}

	f, err := file.Open()
	if err != nil {
		return fmt.Errorf("no se pudo leer el archivo")
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	detected := http.DetectContentType(buf[:n])

	if !kind.allowedContentTypes[detected] {
		return fmt.Errorf("tipo de archivo no permitido para %s (detectado: %s)", kind.label, detected)
	}

	return nil
}
