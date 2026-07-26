// Package storage aísla dónde viven los archivos subidos (documentos de
// pacientes, INE, logos/avatares, recetas en PDF) detrás de una interfaz
// chica, igual que internal/googlecalendar aísla la API de Google.
//
// Sin AWS_S3_BUCKET configurado, cae de vuelta al disco local (./uploads),
// el mismo comportamiento que tenía la app antes de esta integración — así
// no hace falta tener credenciales de AWS para correr el proyecto en local.
// Con AWS_S3_BUCKET configurado, todo se sube a un bucket S3 privado y las
// URLs que se le devuelven al frontend son firmadas y temporales: nadie
// puede acceder a un archivo sin pasar antes por el backend (que ya valida
// que el doctor sea dueño del paciente/cita), a diferencia de servirlo como
// estático público sin autenticación.
package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Client abstrae guardar/borrar/leer archivos, sin importar el backend real.
type Client interface {
	// Save sube un archivo bajo "key" (ej. "profiles/doc_2_avatar_123.png")
	// y devuelve la referencia que debe guardarse en la base de datos.
	Save(ctx context.Context, key string, fileHeader *multipart.FileHeader) (storedRef string, err error)
	// Delete borra el archivo asociado a una referencia ya guardada.
	Delete(ctx context.Context, storedRef string) error
	// URL devuelve una URL lista para usar por el frontend a partir de una
	// referencia guardada. Puede ser relativa (disco local) o absoluta y
	// temporal (S3 firmado) — el frontend ya sabe distinguir ambos casos.
	URL(ctx context.Context, storedRef string) (string, error)
}

// Config lee la configuración de S3 de variables de entorno.
type Config struct {
	Bucket string
	Region string
}

func LoadConfigFromEnv() Config {
	return Config{
		Bucket: os.Getenv("AWS_S3_BUCKET"),
		Region: os.Getenv("AWS_REGION"),
	}
}

func (c Config) IsConfigured() bool {
	return c.Bucket != "" && c.Region != ""
}

// presignExpiry: cuánto dura viva una URL firmada antes de que haya que
// volver a pedirla (siempre se genera una nueva en cada respuesta del API,
// así que esto solo limita cuánto puede quedarse abierta una pestaña/enlace).
const presignExpiry = 20 * time.Minute

// NewClient decide entre S3 (si está configurado) y disco local (si no),
// sin que el resto del código tenga que saber cuál es cuál.
func NewClient(ctx context.Context, cfg Config) (Client, error) {
	if !cfg.IsConfigured() {
		return &localDiskClient{baseDir: "./uploads"}, nil
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("error al cargar credenciales de AWS: %w", err)
	}

	api := s3.NewFromConfig(awsCfg)
	return &s3Client{
		api:     api,
		presign: s3.NewPresignClient(api),
		bucket:  cfg.Bucket,
	}, nil
}

// ---------------------------------------------------------------------
// Disco local (comportamiento histórico de la app, usado sin AWS_S3_BUCKET)
// ---------------------------------------------------------------------

type localDiskClient struct {
	baseDir string
}

func (l *localDiskClient) Save(ctx context.Context, key string, fileHeader *multipart.FileHeader) (string, error) {
	fullPath := filepath.Join(l.baseDir, key)
	if err := os.MkdirAll(filepath.Dir(fullPath), os.ModePerm); err != nil {
		return "", fmt.Errorf("no se pudo preparar el directorio de subidas: %w", err)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	return "/uploads/" + filepath.ToSlash(key), nil
}

func (l *localDiskClient) Delete(ctx context.Context, storedRef string) error {
	rel := strings.TrimPrefix(storedRef, "/uploads/")
	err := os.Remove(filepath.Join(l.baseDir, rel))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (l *localDiskClient) URL(ctx context.Context, storedRef string) (string, error) {
	// Ya es un path público relativo servido por r.Static("/uploads", ...);
	// el frontend le antepone su propio origen del backend.
	return storedRef, nil
}

// ---------------------------------------------------------------------
// S3 (usado cuando AWS_S3_BUCKET y AWS_REGION están configurados)
// ---------------------------------------------------------------------

type s3Client struct {
	api     *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func (s *s3Client) Save(ctx context.Context, key string, fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	contentType := fileHeader.Header.Get("Content-Type")
	_, err = s.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        file,
		ContentType: &contentType,
	})
	if err != nil {
		return "", fmt.Errorf("error al subir el archivo a S3: %w", err)
	}

	// Guardamos solo la key: el bucket es privado, así que una URL directa
	// no serviría de nada sin firmar.
	return key, nil
}

func (s *s3Client) Delete(ctx context.Context, storedRef string) error {
	_, err := s.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &storedRef,
	})
	return err
}

func (s *s3Client) URL(ctx context.Context, storedRef string) (string, error) {
	if storedRef == "" {
		return "", nil
	}
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &storedRef,
	}, s3.WithPresignExpires(presignExpiry))
	if err != nil {
		return "", fmt.Errorf("error al firmar la URL de S3: %w", err)
	}
	return req.URL, nil
}
