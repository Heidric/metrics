package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Heidric/metrics.git/internal/errors"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/model"
	"github.com/go-chi/chi"
	"github.com/rs/zerolog"
)

type mockMetrics struct {
	updateGaugeFn        func(name, value string) error
	updateCounterFn      func(name, value string) error
	getMetricFn          func(metricType, metricName string) (string, error)
	listMetricsFn        func() map[string]string
	updateMetricJSONFn   func(metric *model.Metrics) error
	getMetricJSONFn      func(metric *model.Metrics) error
	updateMetricsBatchFn func(metrics []*model.Metrics) error
}

func (m *mockMetrics) Ping(ctx context.Context) error {
	return nil
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

func (m *mockMetrics) UpdateMetricJSON(metric *model.Metrics) error {
	return m.updateMetricJSONFn(metric)
}

func (m *mockMetrics) GetMetricJSON(metric *model.Metrics) error {
	return m.getMetricJSONFn(metric)
}

func (m *mockMetrics) UpdateMetricsBatch(metrics []*model.Metrics) error {
	if m.updateMetricsBatchFn != nil {
		return m.updateMetricsBatchFn(metrics)
	}
	return nil
}

func ptrFloat64(v float64) *float64 { return &v }

func ptrInt64(v int64) *int64 { return &v }

func TestHandlers(t *testing.T) {
	testLogger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.Disabled)
	logger.Log = &testLogger

	t.Run("UpdateGauge success", func(t *testing.T) {
		mock := &mockMetrics{
			updateGaugeFn: func(name, value string) error {
				return nil
			},
		}

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

	t.Run("UpdateMetricJSON gauge success", func(t *testing.T) {
		mock := &mockMetrics{
			updateMetricJSONFn: func(metric *model.Metrics) error {
				return nil
			},
		}

		srv := NewServer(":8080", mock)
		r := srv.Srv.Handler.(*chi.Mux)

		metric := model.Metrics{
			ID:    "temp",
			MType: "gauge",
			Value: new(float64),
		}
		*metric.Value = 42.5

		body, _ := json.Marshal(metric)
		req := httptest.NewRequest("POST", "/update/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response model.Metrics
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal("failed to unmarshal response")
		}

		if response.ID != "temp" || *response.Value != 42.5 {
			t.Error("response doesn't match request")
		}
	})

	t.Run("UpdateMetricJSON counter success", func(t *testing.T) {
		mock := &mockMetrics{
			updateMetricJSONFn: func(metric *model.Metrics) error {
				*metric.Delta = 10
				return nil
			},
		}

		srv := NewServer(":8080", mock)
		r := srv.Srv.Handler.(*chi.Mux)

		metric := model.Metrics{
			ID:    "counter",
			MType: "counter",
			Delta: new(int64),
		}
		*metric.Delta = 5

		body, _ := json.Marshal(metric)
		req := httptest.NewRequest("POST", "/update/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response model.Metrics
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal("failed to unmarshal response")
		}

		if response.ID != "counter" || *response.Delta != 10 {
			t.Error("response doesn't match expected value")
		}
	})

	t.Run("UpdateMetricJSON invalid type", func(t *testing.T) {
		mock := &mockMetrics{
			updateMetricJSONFn: func(metric *model.Metrics) error {
				return errors.ErrInvalidType
			},
		}

		srv := NewServer(":8080", mock)
		r := srv.Srv.Handler.(*chi.Mux)

		metric := model.Metrics{
			ID:    "invalid",
			MType: "unknown",
		}

		body, _ := json.Marshal(metric)
		req := httptest.NewRequest("POST", "/update/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var response errors.CommonError
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal("failed to unmarshal error response")
		}

		if response.Title != "Validation error occurred" {
			t.Error("unexpected error response")
		}
	})

	t.Run("GetMetricJSON success", func(t *testing.T) {
		mock := &mockMetrics{
			getMetricJSONFn: func(metric *model.Metrics) error {
				if metric.MType == "gauge" {
					value := 99.9
					metric.Value = &value
				}
				return nil
			},
		}

		srv := NewServer(":8080", mock)
		r := srv.Srv.Handler.(*chi.Mux)

		metric := model.Metrics{
			ID:    "test",
			MType: "gauge",
		}

		body, _ := json.Marshal(metric)
		req := httptest.NewRequest("POST", "/value/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var response model.Metrics
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal("failed to unmarshal response")
		}

		if *response.Value != 99.9 {
			t.Error("response value doesn't match expected")
		}
	})

	t.Run("GetMetricJSON not found", func(t *testing.T) {
		mock := &mockMetrics{
			getMetricJSONFn: func(metric *model.Metrics) error {
				return errors.ErrKeyNotFound
			},
		}

		srv := NewServer(":8080", mock)
		r := srv.Srv.Handler.(*chi.Mux)

		metric := model.Metrics{
			ID:    "missing",
			MType: "gauge",
		}

		body, _ := json.Marshal(metric)
		req := httptest.NewRequest("POST", "/value/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}

		var response errors.CommonError
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal("failed to unmarshal error response")
		}

		if response.Title != "Not found" {
			t.Error("unexpected error response")
		}
	})

	t.Run("UpdateMetricsBatch success", func(t *testing.T) {
		mock := &mockMetrics{
			updateMetricsBatchFn: func(metrics []*model.Metrics) error {
				if len(metrics) != 2 {
					t.Errorf("expected 2 metrics, got %d", len(metrics))
				}
				return nil
			},
		}

		srv := NewServer(":8080", mock)
		r := srv.Srv.Handler.(*chi.Mux)

		metrics := []*model.Metrics{
			{ID: "batchGauge", MType: "gauge", Value: ptrFloat64(3.14)},
			{ID: "batchCounter", MType: "counter", Delta: ptrInt64(42)},
		}

		jsonData, err := json.Marshal(metrics)
		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		if _, err := gz.Write(jsonData); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(buf.Bytes()))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
	})
}
