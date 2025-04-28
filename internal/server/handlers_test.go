package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi"
)

type mockMetrics struct {
	updateGaugeFn   func(name, value string) error
	updateCounterFn func(name, value string) error
	getMetricFn     func(metricType, metricName string) (string, error)
	listMetricsFn   func() map[string]string
}

func (m *mockMetrics) UpdateGauge(name, value string) error {
	return m.updateGaugeFn(name, value)
}

func (m *mockMetrics) UpdateCounter(name, value string) error {
	return m.updateCounterFn(name, value)
}

func (m *mockMetrics) GetMetric(metricType, metricName string) (string, error) {
	return m.getMetricFn(metricType, metricName)
}

func (m *mockMetrics) ListMetrics() map[string]string {
	return m.listMetricsFn()
}

func TestHandlers(t *testing.T) {
	t.Run("UpdateGauge success", func(t *testing.T) {
		mock := &mockMetrics{
			updateGaugeFn: func(name, value string) error {
				return nil
			},
		}

		// Создаем полноценный сервер с chi роутером
		srv := NewServer(":8080", mock)
		r := srv.Srv.Handler.(*chi.Mux)

		req := httptest.NewRequest("POST", "/update/gauge/temp/42.5", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("ListMetrics success", func(t *testing.T) {
		mock := &mockMetrics{
			listMetricsFn: func() map[string]string {
				return map[string]string{"temp": "42.5"}
			},
		}

		srv := NewServer(":8080", mock)
		r := srv.Srv.Handler.(*chi.Mux)

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "temp") {
			t.Error("response should contain metric")
		}
	})
}
