package services

import (
	"testing"

	"github.com/Heidric/metrics.git/internal/errors"
)

type mockStorage struct {
	data map[string]string
}

func (m *mockStorage) Set(key, value string) {
	m.data[key] = value
}

func (m *mockStorage) Get(key string) (string, error) {
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return "", errors.ErrKeyNotFound
}

func (m *mockStorage) GetAll() map[string]string {
	result := make(map[string]string)
	for k, v := range m.data {
		result[k] = v
	}
	return result
}

func TestMetricsService(t *testing.T) {
	t.Run("UpdateGauge", func(t *testing.T) {
		storage := &mockStorage{data: make(map[string]string)}
		service := NewMetricsService(storage)

		err := service.UpdateGauge("temp", "42.5")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if storage.data["temp"] != "42.5" {
			t.Errorf("expected 42.5, got %s", storage.data["temp"])
		}
	})

	t.Run("UpdateCounter", func(t *testing.T) {
		storage := &mockStorage{data: make(map[string]string)}
		service := NewMetricsService(storage)

		err := service.UpdateCounter("hits", "10")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = service.UpdateCounter("hits", "5")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if storage.data["hits"] != "15" {
			t.Errorf("expected 15, got %s", storage.data["hits"])
		}
	})
}
