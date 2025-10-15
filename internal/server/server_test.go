package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/Heidric/metrics.git/internal/crypto"
	"github.com/Heidric/metrics.git/internal/customerrors"
	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/model"
	"github.com/Heidric/metrics.git/internal/services"
	"github.com/rs/zerolog"
)

type stubMetrics struct {
	getErr        error
	jsonGetErr    error
	jsonUpdateErr error
	batchErr      error
	pingErr       error

	list   map[string]string
	getVal string
}

type counterValidatingStub struct{ stubMetrics }

func (s *counterValidatingStub) UpdateCounter(name, value string) error {
	if _, err := strconv.ParseInt(value, 10, 64); err != nil {
		return customerrors.ErrInvalidValue
	}
	return nil
}
func (s *stubMetrics) ListMetrics() map[string]string { return s.list }
func (s *stubMetrics) GetMetric(metricType, metricName string) (string, error) {
	return s.getVal, s.getErr
}
func (s *stubMetrics) UpdateGauge(name, value string) error         { return nil }
func (s *stubMetrics) UpdateCounter(name, value string) error       { return nil }
func (s *stubMetrics) UpdateMetricJSON(metric *model.Metrics) error { return s.jsonUpdateErr }
func (s *stubMetrics) GetMetricJSON(metric *model.Metrics) error {
	if s.jsonGetErr != nil {
		return s.jsonGetErr
	}
	if metric.MType == model.GaugeType {
		metric.Value = floatPtr(1.23)
	} else {
		metric.Delta = intPtr(7)
	}
	return nil
}
func (s *stubMetrics) UpdateMetricsBatch(metrics []*model.Metrics) error { return s.batchErr }
func (s *stubMetrics) Ping(ctx context.Context) error                    { return s.pingErr }

func floatPtr(f float64) *float64 { return &f }
func intPtr(i int64) *int64       { return &i }

func TestServerRoutes(t *testing.T) {
	ctx := context.Background()
	testLogger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.Disabled)
	logger.Log = &testLogger

	hashKey := "hash-key"

	tmpFile, err := os.CreateTemp("", "testdb-")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	storage := db.NewStore("", 0)
	defer storage.Close()

	err = storage.SetGauge(ctx, "temp", 42.5)
	if err != nil {
		t.Fatal(err)
	}
	err = storage.SetCounter(ctx, "requests", 10)
	if err != nil {
		t.Fatal(err)
	}

	service := services.NewMetricsService(storage)
	srv := NewServer(":8080", hashKey, service)
	testServer := httptest.NewServer(srv.Srv.Handler)
	defer testServer.Close()

	tests := []struct {
		name          string
		method        string
		path          string
		wantHeader    string
		wantHeaderVal string

		wantStatus int
	}{
		{
			name:       "Update gauge - valid",
			method:     "POST",
			path:       "/update/gauge/temp/42.5",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Update counter - valid",
			method:     "POST",
			path:       "/update/counter/requests/1",
			wantStatus: http.StatusOK,
		},
		{
			name:          "List metrics",
			method:        "GET",
			path:          "/",
			wantStatus:    http.StatusOK,
			wantHeader:    "Content-Type",
			wantHeaderVal: "text/html",
		},
		{
			name:          "Get metric - valid",
			method:        "GET",
			path:          "/value/gauge/temp",
			wantStatus:    http.StatusOK,
			wantHeader:    "Content-Type",
			wantHeaderVal: "text/plain",
		},
		{
			name:       "Not found",
			method:     "GET",
			path:       "/unknown",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Ping handler",
			method:     "GET",
			path:       "/ping",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, testServer.URL+tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}

			if tt.wantHeader != "" {
				if val := resp.Header.Get(tt.wantHeader); val != tt.wantHeaderVal {
					t.Errorf("Expected header %s: %s, got %s", tt.wantHeader, tt.wantHeaderVal, val)
				}
			}
		})
	}
}

func TestUpdateMetricJSON_InvalidJSON_Returns400(t *testing.T) {
	stub := &stubMetrics{}
	srv := NewServer(":0", "k", stub)

	r := srv.GetRouter()
	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rr.Code)
	}
}

func TestGetMetricJSON_NotFound_404(t *testing.T) {
	stub := &stubMetrics{jsonGetErr: customerrors.ErrKeyNotFound}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	m := model.Metrics{ID: "nope", MType: model.GaugeType}
	body, _ := json.Marshal(m)

	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rr.Code)
	}
}

func TestGetMetricJSON_OK_HashHeaderSet(t *testing.T) {
	stub := &stubMetrics{}
	hashKey := "secret"
	srv := NewServer(":0", hashKey, stub)
	r := srv.GetRouter()

	m := model.Metrics{ID: "cpu", MType: model.GaugeType}
	body, _ := json.Marshal(m)

	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
	respBody := rr.Body.Bytes()
	exp := crypto.HashSHA256(respBody, hashKey)
	if h := rr.Header().Get("HashSHA256"); h != exp && rr.Header().Get("Hash") != exp {
		t.Fatalf("hash header missing or wrong: got HashSHA256=%q Hash=%q want %q",
			rr.Header().Get("HashSHA256"), rr.Header().Get("Hash"), exp)
	}
}

func TestGzipMiddleware_CompressesWhenAccepted(t *testing.T) {
	stub := &stubMetrics{list: map[string]string{"a": "1", "b": "2"}}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", rr.Header().Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer zr.Close()
	if _, err := io.ReadAll(zr); err != nil {
		t.Fatalf("decompress body: %v", err)
	}
}

func TestUpdateMetric_Path_InvalidType_Returns400(t *testing.T) {
	stub := &stubMetrics{}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/unknown/alloc/1", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rr.Code)
	}
}

func TestUpdateMetric_Path_Gauge_OK(t *testing.T) {
	stub := &stubMetrics{}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/alloc/123.45", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
}

func TestUpdateMetric_Path_Counter_InvalidNumber_Returns400(t *testing.T) {
	stub := &counterValidatingStub{}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/counter/reqs/notANumber", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rr.Code)
	}
}

func TestUpdatesBatch_InvalidJSON_400(t *testing.T) {
	stub := &stubMetrics{}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rr.Code)
	}
}

func TestUpdatesBatch_BadRequestError(t *testing.T) {
	stub := &stubMetrics{batchErr: io.ErrUnexpectedEOF}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	metrics := []*model.Metrics{{ID: "a", MType: model.GaugeType, Value: floatPtr(1)}}
	body, _ := json.Marshal(metrics)

	req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rr.Code)
	}
}

func TestPing_OK_200(t *testing.T) {
	stub := &stubMetrics{}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
}

func TestPing_Error_500(t *testing.T) {
	stub := &stubMetrics{pingErr: io.ErrUnexpectedEOF}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", rr.Code)
	}
}

func TestGzipMiddleware_NotAppliedWithoutAcceptEncoding(t *testing.T) {
	stub := &stubMetrics{list: map[string]string{"x": "1"}}
	srv := NewServer(":0", "k", stub)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Header().Get("Content-Encoding") == "gzip" {
		t.Fatalf("unexpected gzip without Accept-Encoding")
	}
}
