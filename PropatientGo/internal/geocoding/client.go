// Package geocoding convierte una dirección de texto libre en coordenadas
// (lat/lng). Usa LocationIQ si hay LOCATIONIQ_API_KEY configurada (pensado
// para llamarse desde un servidor, con cuota generosa); si no, cae de
// vuelta a Nominatim, el servicio gratuito de OpenStreetMap que no pide API
// key pero que en la práctica bloquea o limita a servidores en la nube
// (Render comparte IPs con muchas otras apps que ya agotaron la cuota de
// Nominatim para esas direcciones) — ver el 503 recurrente reportado en
// producción que motivó agregar LocationIQ como proveedor principal.
package geocoding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

// Client permite mockear la geocodificación en tests de integración sin
// depender de la red ni de la disponibilidad de Nominatim.
type Client interface {
	// Geocode devuelve nil (sin error) si la dirección no se pudo ubicar
	// en el mapa — no es un error del sistema, solo "no encontrado".
	Geocode(ctx context.Context, address string) (*Coordinates, error)
}

type nominatimClient struct {
	httpClient *http.Client
}

// NewClient arma el cliente de geocodificación a partir de las variables de
// entorno: con LOCATIONIQ_API_KEY configurada usa LocationIQ (recomendado,
// pensado para llamarse desde un servidor); sin ella cae de vuelta a
// Nominatim para no romper instalaciones que no la hayan configurado.
func NewClient() Client {
	if apiKey := os.Getenv("LOCATIONIQ_API_KEY"); apiKey != "" {
		return &locationIQClient{apiKey: apiKey, httpClient: &http.Client{Timeout: 8 * time.Second}}
	}
	return &nominatimClient{httpClient: &http.Client{Timeout: 8 * time.Second}}
}

type nominatimResult struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

func (n *nominatimClient) Geocode(ctx context.Context, address string) (*Coordinates, error) {
	if address == "" {
		return nil, errors.New("dirección vacía")
	}

	endpoint := "https://nominatim.openstreetmap.org/search?format=json&limit=1&q=" + url.QueryEscape(address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	// Nominatim exige un User-Agent identificable; sin esto puede bloquear
	// las peticiones sin previo aviso.
	req.Header.Set("User-Agent", "ProPatientApp/1.0 (consultorio medico, geocoding de perfiles publicos)")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("nominatim respondió con un error")
	}

	var results []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	lat, err := strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return nil, err
	}
	lon, err := strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return nil, err
	}

	return &Coordinates{Latitude: lat, Longitude: lon}, nil
}

// locationIQClient usa la misma forma de respuesta que Nominatim (LocationIQ
// es un wrapper comercial sobre datos de OpenStreetMap), solo cambia el
// endpoint y que la key va en la URL en vez del User-Agent.
type locationIQClient struct {
	apiKey     string
	httpClient *http.Client
}

func (l *locationIQClient) Geocode(ctx context.Context, address string) (*Coordinates, error) {
	if address == "" {
		return nil, errors.New("dirección vacía")
	}

	endpoint := "https://us1.locationiq.com/v1/search?format=json&limit=1&key=" + url.QueryEscape(l.apiKey) + "&q=" + url.QueryEscape(address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// LocationIQ responde 404 (con cuerpo JSON de error) cuando no
	// encuentra la dirección, a diferencia de Nominatim que responde 200
	// con un arreglo vacío — se trata igual como "no encontrado", no como
	// error del sistema.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("locationiq respondió con un error")
	}

	var results []nominatimResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	lat, err := strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return nil, err
	}
	lon, err := strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return nil, err
	}

	return &Coordinates{Latitude: lat, Longitude: lon}, nil
}

// ResolveCoordinates decide la posición final a guardar para un doctor,
// compartida entre el registro inicial (CompleteProfile.tsx) y el perfil
// (DoctorProfile.tsx), ambos con un mapa donde el doctor puede arrastrar el
// pin para corregir la ubicación automática.
//
// Prioridad: 1) posición manual (el doctor arrastró el pin en el mapa,
// llega como manualLatStr/manualLngStr) siempre gana, sin volver a
// geocodificar; 2) si la dirección no cambió y ya había coordenadas, se
// dejan igual, para no golpear la API en cada guardado; 3) si no, se
// geocodifica la dirección nueva.
func ResolveCoordinates(ctx context.Context, client Client, address, previousAddress string, existingLat, existingLng *float64, manualLatStr, manualLngStr string) (lat *float64, lng *float64, err error) {
	if manualLatStr != "" && manualLngStr != "" {
		mLat, errLat := strconv.ParseFloat(manualLatStr, 64)
		mLng, errLng := strconv.ParseFloat(manualLngStr, 64)
		if errLat == nil && errLng == nil {
			return &mLat, &mLng, nil
		}
	}

	if address == "" {
		return existingLat, existingLng, nil
	}
	if address == previousAddress && existingLat != nil {
		return existingLat, existingLng, nil
	}

	coords, geoErr := client.Geocode(ctx, address)
	if geoErr != nil {
		return existingLat, existingLng, geoErr
	}
	if coords == nil {
		return existingLat, existingLng, nil
	}
	return &coords.Latitude, &coords.Longitude, nil
}
