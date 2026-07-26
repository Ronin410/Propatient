package storage

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildMultipartFile arma una petición HTTP real en memoria y la parsea,
// para obtener un *multipart.FileHeader genuino (con su propio método
// Open()) en vez de construir el struct a mano.
func buildMultipartFile(t *testing.T, fieldName, fileName string, content []byte) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("error creando el form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("error escribiendo el contenido: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("error cerrando el writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("error parseando el multipart form: %v", err)
	}

	return req.MultipartForm.File[fieldName][0]
}

func TestValidateUploadedFile_AcceptsValidImage(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	file := buildMultipartFile(t, "avatar", "foto.png", png)

	if err := ValidateUploadedFile(file, UploadKindAvatarOrLogo); err != nil {
		t.Fatalf("esperaba que una imagen PNG válida pasara, obtuve: %v", err)
	}
}

func TestValidateUploadedFile_RejectsWrongTypeMasqueradingAsImage(t *testing.T) {
	// Un ejecutable con extensión .png: el Content-Type que manda el
	// navegador no importa, la validación real es por contenido.
	fakeExe := []byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00")
	file := buildMultipartFile(t, "avatar", "virus.png", fakeExe)

	err := ValidateUploadedFile(file, UploadKindAvatarOrLogo)
	if err == nil {
		t.Fatal("esperaba que un archivo que no es una imagen real fuera rechazado")
	}
}

func TestValidateUploadedFile_RejectsFileOverSizeLimit(t *testing.T) {
	oversized := bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47}, (4<<20)/4) // ~4 MiB, PNG signature repeated
	file := buildMultipartFile(t, "avatar", "grande.png", oversized)

	err := ValidateUploadedFile(file, UploadKindAvatarOrLogo) // límite: 3 MiB
	if err == nil {
		t.Fatal("esperaba que un archivo de ~4MB fuera rechazado (límite 3MB)")
	}
	if !strings.Contains(err.Error(), "MB") {
		t.Fatalf("esperaba un mensaje sobre el límite de tamaño, obtuve: %v", err)
	}
}

func TestValidateUploadedFile_AcceptsPDFForMedicalDocument(t *testing.T) {
	pdf := []byte("%PDF-1.4\n%âãÏÓ\n1 0 obj\n")
	file := buildMultipartFile(t, "files", "receta.pdf", pdf)

	if err := ValidateUploadedFile(file, UploadKindMedicalDocument); err != nil {
		t.Fatalf("esperaba que un PDF válido pasara para documentos médicos, obtuve: %v", err)
	}
}

func TestValidateUploadedFile_RejectsPDFForAvatarKind(t *testing.T) {
	pdf := []byte("%PDF-1.4\n%âãÏÓ\n1 0 obj\n")
	file := buildMultipartFile(t, "avatar", "documento.pdf", pdf)

	err := ValidateUploadedFile(file, UploadKindAvatarOrLogo)
	if err == nil {
		t.Fatal("un PDF no debe aceptarse como avatar/logo (solo imágenes)")
	}
}
