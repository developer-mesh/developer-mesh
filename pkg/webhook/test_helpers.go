package webhook

import (
	"context"
	"strings"
	"sync"
)

// mockStorageBackend is a mock implementation of StorageBackend for testing
type mockStorageBackend struct {
	data map[string][]byte
	mu   sync.Mutex
}

func (m *mockStorageBackend) Store(ctx context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	m.data[key] = data
	return nil
}

func (m *mockStorageBackend) Retrieve(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return data, nil
}

func (m *mockStorageBackend) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *mockStorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keys []string
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *mockStorageBackend) Close() error {
	return nil
}
