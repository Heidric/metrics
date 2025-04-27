package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Heidric/metrics.git/internal/errors"
	"github.com/Heidric/metrics.git/internal/services"
)

type MockStorage struct {
	data           map[string]string
	updateGaugeErr error
	getErr         error
}

func (m *MockStorage) Set(key, value string) {
	m.data[key] = value
}

func (m *MockStorage) Get(key string) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return "", errors.ErrKeyNotFound
}

func (m *MockStorage) Delete(key string) {
	delete(m.data, key)
}

func (m *MockStorage) GetAll() map[string]string {
	return m.data
}

func (m *MockStorage) Close() {}

func TestMetricsHandlers(t *testing.T) {
	t.Run("UpdateMetricHandler gauge success", func(t *testing.T) {
		storage := &MockStorage{data: make(map[string]string)}
		service := services.NewMetricsService(storage)
		handler := NewMetricsHandlers(service)

		req := httptest.NewRequest("POST", "/update/gauge/temp/42", nil)
		w := httptest.NewRecorder()

		handler.UpdateMetricHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("UpdateMetricHandler counter success", func(t *testing.T) {
		storage := &MockStorage{data: make(map[string]string)}
		service := services.NewMetricsService(storage)
		handler := NewMetricsHandlers(service)

		req := httptest.NewRequest("POST", "/update/counter/hits/10", nil)
		w := httptest.NewRecorder()

		handler.UpdateMetricHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("UpdateMetricHandler invalid method", func(t *testing.T) {
		storage := &MockStorage{data: make(map[string]string)}
		service := services.NewMetricsService(storage)
		handler := NewMetricsHandlers(service)

		req := httptest.NewRequest("GET", "/update/gauge/temp/42", nil)
		w := httptest.NewRecorder()

		handler.UpdateMetricHandler(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("Expected status 405, got %d", w.Code)
		}
	})

	t.Run("UpdateMetricHandler invalid metric type", func(t *testing.T) {
		storage := &MockStorage{data: make(map[string]string)}
		service := services.NewMetricsService(storage)
		handler := NewMetricsHandlers(service)

		req := httptest.NewRequest("POST", "/update/invalid/temp/42", nil)
		w := httptest.NewRecorder()

		handler.UpdateMetricHandler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected status 400, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Validation error occurred") {
			t.Fatal("Expected validation error message")
		}
	})

	t.Run("UpdateMetricHandler gauge invalid value", func(t *testing.T) {
		storage := &MockStorage{
			data:           make(map[string]string),
			updateGaugeErr: errors.ErrInvalidValue,
		}
		service := services.NewMetricsService(storage)
		handler := NewMetricsHandlers(service)

		req := httptest.NewRequest("POST", "/update/gauge/temp/invalid", nil)
		w := httptest.NewRecorder()

		handler.UpdateMetricHandler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected status 400, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Validation error occurred") {
			t.Fatal("Expected validation error message")
		}
	})

	t.Run("UpdateMetricHandler counter invalid value", func(t *testing.T) {
		storage := &MockStorage{
			data:           make(map[string]string),
			updateGaugeErr: errors.ErrInvalidValue,
		}
		service := services.NewMetricsService(storage)
		handler := NewMetricsHandlers(service)

		req := httptest.NewRequest("POST", "/update/counter/hits/invalid", nil)
		w := httptest.NewRecorder()

		handler.UpdateMetricHandler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Expected status 400, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Validation error occurred") {
			t.Fatal("Expected validation error message")
		}
	})

	t.Run("UpdateMetricHandler internal error", func(t *testing.T) {
		storage := &MockStorage{
			data: map[string]string{
				"temp": "invalid",
			},
			getErr: nil,
		}
		service := services.NewMetricsService(storage)
		handler := NewMetricsHandlers(service)

		req := httptest.NewRequest("POST", "/update/counter/temp/42", nil)
		w := httptest.NewRecorder()

		handler.UpdateMetricHandler(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("Expected status 500, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Resource temporarily unavailable") {
			t.Fatal("Expected internal error message")
		}
	})

	t.Run("NotFoundHandler", func(t *testing.T) {
		storage := &MockStorage{data: make(map[string]string)}
		service := services.NewMetricsService(storage)
		handler := NewMetricsHandlers(service)

		req := httptest.NewRequest("GET", "/nonexistent", nil)
		w := httptest.NewRecorder()

		handler.NotFoundHandler(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("Expected status 404, got %d", w.Code)
		}
	})
}
