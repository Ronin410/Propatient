package server_test

import (
	"context"
	"mime/multipart"
	"sync"

	"propatient-api/internal/storage"
)

// mockStorageClient simula el backend de archivos sin tocar disco ni red:
// registra qué se guardó/borró/firmó, y devuelve URLs "firmadas" con un
// prefijo reconocible para poder afirmar sobre ellas en los tests.
type mockStorageClient struct {
	mu sync.Mutex

	savedKeys   []string
	deletedRefs []string
	urlCalls    []string
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{}
}

func (m *mockStorageClient) Save(ctx context.Context, key string, fileHeader *multipart.FileHeader) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.savedKeys = append(m.savedKeys, key)
	return key, nil
}

func (m *mockStorageClient) Delete(ctx context.Context, storedRef string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedRefs = append(m.deletedRefs, storedRef)
	return nil
}

func (m *mockStorageClient) URL(ctx context.Context, storedRef string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.urlCalls = append(m.urlCalls, storedRef)
	if storedRef == "" {
		return "", nil
	}
	return "https://mock-presigned.example.com/" + storedRef, nil
}

func (m *mockStorageClient) savedKeyCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.savedKeys)
}

var _ storage.Client = (*mockStorageClient)(nil)
