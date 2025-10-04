package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Heidric/metrics.git/internal/crypto"
	"github.com/Heidric/metrics.git/internal/customerrors"
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

func (m *mockMetrics) Ping(ctx context.Context) error         { return nil }
func (m *mockMetrics) UpdateGauge(name, value string) error   { return m.updateGaugeFn(name, value) }
func (m *mockMetrics) UpdateCounter(name, value string) error { return m.updateCounterFn(name, value) }
func (m *mockMetrics) GetMetric(t, n string) (string, error)  { return m.getMetricFn(t, n) }
func (m *mockMetrics) ListMetrics() map[string]string         { return m.listMetricsFn() }
func (m *mockMetrics) UpdateMetricJSON(metric *model.Metrics) error {
	return m.updateMetricJSONFn(metric)
}
func (m *mockMetrics) GetMetricJSON(metric *model.Metrics) error { return m.getMetricJSONFn(metric) }
func (m *mockMetrics) UpdateMetricsBatch(metrics []*model.Metrics) error {
	if m.updateMetricsBatchFn != nil {
		return m.updateMetricsBatchFn(metrics)
	}
	return nil
}

func newTestServer(t *testing.T, metrics *mockMetrics, hashKey string) (*chi.Mux, *Server) {
	t.Helper()
	l := zerolog.New(nil).Level(zerolog.Disabled)
	logger.Log = &l
	srv := NewServer(":8080", hashKey, metrics)
	return srv.Srv.Handler.(*chi.Mux), srv
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }

func gzipCompress(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	if _, err := gzw.Write(raw); err != nil {
		t.Fatalf("gzip write failed: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("gzip close failed: %v", err)
	}
	return buf.Bytes()
}

func TestHandlers(t *testing.T) {
	t.Run("UpdateMetricJSON gauge success", func(t *testing.T) {
		mock := &mockMetrics{
			updateMetricJSONFn: func(metric *model.Metrics) error { return nil },
		}
		r, _ := newTestServer(t, mock, "")
		metric := model.Metrics{ID: "temp", MType: model.GaugeType, Value: ptrFloat64(42.5)}
		body, _ := json.Marshal(metric)

		req := httptest.NewRequest("POST", "/update/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("UpdateMetricJSON counter success", func(t *testing.T) {
		mock := &mockMetrics{
			updateMetricJSONFn: func(metric *model.Metrics) error {
				*metric.Delta = 10
				return nil
			},
		}
		r, _ := newTestServer(t, mock, "")
		metric := model.Metrics{ID: model.CounterType, MType: model.CounterType, Delta: ptrInt64(5)}
		body, _ := json.Marshal(metric)

		req := httptest.NewRequest("POST", "/update/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("UpdateMetricJSON invalid type", func(t *testing.T) {
		mock := &mockMetrics{
			updateMetricJSONFn: func(metric *model.Metrics) error {
				return customerrors.ErrInvalidType
			},
		}
		r, _ := newTestServer(t, mock, "")
		metric := model.Metrics{ID: "invalid", MType: "bad"}
		body, _ := json.Marshal(metric)

		req := httptest.NewRequest("POST", "/update/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
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
		r, _ := newTestServer(t, mock, "")
		metrics := []*model.Metrics{
			{ID: "m1", MType: model.GaugeType, Value: ptrFloat64(3.14)},
			{ID: "m2", MType: model.CounterType, Delta: ptrInt64(1)},
		}
		raw, _ := json.Marshal(metrics)

		req := httptest.NewRequest("POST", "/updates/", bytes.NewReader(gzipCompress(t, raw)))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("UpdateMetricJSON with valid hash", func(t *testing.T) {
		key := "secret"
		mock := &mockMetrics{
			updateMetricJSONFn: func(metric *model.Metrics) error { return nil },
		}
		r, _ := newTestServer(t, mock, key)

		metric := model.Metrics{ID: "x", MType: model.GaugeType, Value: ptrFloat64(1.0)}
		data, _ := json.Marshal(metric)
		hash := crypto.HashSHA256(data, key)

		req := httptest.NewRequest("POST", "/update/", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("HashSHA256", hash)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 with valid hash, got %d", w.Code)
		}
	})

	t.Run("UpdateMetricsBatch with valid hash", func(t *testing.T) {
		key := "secret"
		mock := &mockMetrics{
			updateMetricsBatchFn: func(metrics []*model.Metrics) error { return nil },
		}
		r, _ := newTestServer(t, mock, key)

		metrics := []*model.Metrics{
			{ID: "a", MType: model.CounterType, Delta: ptrInt64(5)},
		}
		raw, _ := json.Marshal(metrics)
		hash := crypto.HashSHA256(raw, key)

		req := httptest.NewRequest("POST", "/updates/", bytes.NewReader(gzipCompress(t, raw)))
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("HashSHA256", hash)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 with valid hash, got %d", w.Code)
		}
	})
}
