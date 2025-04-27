package services

import (
	"testing"

	"github.com/Heidric/metrics.git/internal/errors"
)

type MockStorage struct {
	data map[string]string
}

func (m *MockStorage) Set(key, value string) {
	m.data[key] = value
}

func (m *MockStorage) Get(key string) (string, error) {
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return "", errors.ErrKeyNotFound
}

func TestMetricsService(t *testing.T) {
	t.Run("UpdateGauge valid value", func(t *testing.T) {
		storage := &MockStorage{data: make(map[string]string)}
		service := NewMetricsService(storage)

		err := service.UpdateGauge("temp", "42")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if storage.data["temp"] != "42" {
			t.Fatalf("Expected '42', got '%s'", storage.data["temp"])
		}
	})

	t.Run("UpdateGauge invalid value", func(t *testing.T) {
		storage := &MockStorage{data: make(map[string]string)}
		service := NewMetricsService(storage)

		err := service.UpdateGauge("temp", "not_a_number")
		if err != errors.ErrInvalidValue {
			t.Fatalf("Expected ErrInvalidValue, got %v", err)
		}
	})

	t.Run("UpdateCounter valid value", func(t *testing.T) {
		storage := &MockStorage{data: make(map[string]string)}
		service := NewMetricsService(storage)

		err := service.UpdateCounter("hits", "10")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if storage.data["hits"] != "10" {
			t.Fatalf("Expected '10', got '%s'", storage.data["hits"])
		}

		err = service.UpdateCounter("hits", "5")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if storage.data["hits"] != "15" {
			t.Fatalf("Expected '15', got '%s'", storage.data["hits"])
		}
	})

	t.Run("UpdateCounter invalid value", func(t *testing.T) {
		storage := &MockStorage{data: make(map[string]string)}
		service := NewMetricsService(storage)

		err := service.UpdateCounter("hits", "not_a_number")
		if err != errors.ErrInvalidValue {
			t.Fatalf("Expected ErrInvalidValue, got %v", err)
		}
	})

	t.Run("UpdateCounter with existing invalid value", func(t *testing.T) {
		storage := &MockStorage{data: map[string]string{"hits": "invalid"}}
		service := NewMetricsService(storage)

		err := service.UpdateCounter("hits", "10")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
	})
}
