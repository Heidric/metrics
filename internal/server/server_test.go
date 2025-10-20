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
	"github.com/Heidric/metrics.git/internal/server/servertest"
	"github.com/Heidric/metrics.git/internal/services"
	"github.com/rs/zerolog"
)

func floatPtr(f float64) *float64 { return &f }
func int64Ptr(v int64) *int64     { return &v }

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
	mock := servertest.NewMetricsMock()
	srv := NewServer(":0", "k", mock)

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
	mock := servertest.NewMetricsMock(
		servertest.WithGetMetricJSON(func(m *model.Metrics) error {
			return customerrors.ErrKeyNotFound
		}),
	)
	srv := NewServer(":0", "k", mock)
	r := srv.GetRouter()

	body, _ := json.Marshal(model.Metrics{ID: "nope", MType: model.GaugeType})

	req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rr.Code)
	}
}

func TestGetMetricJSON_OK_HashHeaderSet(t *testing.T) {
	hashKey := "secret"
	mock := servertest.NewMetricsMock(
		servertest.WithGetMetricJSON(func(m *model.Metrics) error {
			if m.MType == model.GaugeType {
				v := 1.23
				m.Value = &v
			} else {
				d := int64(7)
				m.Delta = &d
			}
			return nil
		}),
	)
	srv := NewServer(":0", hashKey, mock)
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
	mock := servertest.NewMetricsMock(
		servertest.WithListMetrics(func() map[string]string { return map[string]string{"a": "1", "b": "2"} }),
	)
	srv := NewServer(":0", "k", mock)
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
	mock := servertest.NewMetricsMock()
	srv := NewServer(":0", "k", mock)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/unknown/alloc/1", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rr.Code)
	}
}

func TestUpdateMetric_Path_Gauge_OK(t *testing.T) {
	mock := servertest.NewMetricsMock()
	srv := NewServer(":0", "k", mock)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/gauge/alloc/123.45", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
}

func TestUpdateMetric_Path_Counter_InvalidNumber_Returns400(t *testing.T) {
	mock := servertest.NewMetricsMock(
		servertest.WithUpdateCounter(func(name, value string) error {
			if _, err := strconv.ParseInt(value, 10, 64); err != nil {
				return customerrors.ErrInvalidValue
			}
			return nil
		}),
	)

	srv := NewServer(":0", "k", mock)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodPost, "/update/counter/reqs/notANumber", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rr.Code)
	}
}

func TestUpdatesBatch_InvalidJSON_400(t *testing.T) {
	mock := servertest.NewMetricsMock()
	srv := NewServer(":0", "k", mock)
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
	mock := servertest.NewMetricsMock(
		servertest.WithUpdateMetricsBatch(func(_ []*model.Metrics) error {
			return io.ErrUnexpectedEOF
		}),
	)
	srv := NewServer(":0", "k", mock)
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
	mock := servertest.NewMetricsMock(
		servertest.WithPing(func(ctx context.Context) error { return nil }),
	)
	srv := NewServer(":0", "k", mock)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
}

func TestPing_Error_500(t *testing.T) {
	mock := servertest.NewMetricsMock(
		servertest.WithPing(func(ctx context.Context) error { return io.ErrUnexpectedEOF }),
	)
	srv := NewServer(":0", "k", mock)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", rr.Code)
	}
}

func TestGzipMiddleware_NotAppliedWithoutAcceptEncoding(t *testing.T) {
	mock := servertest.NewMetricsMock(
		servertest.WithListMetrics(func() map[string]string { return map[string]string{"x": "1"} }),
	)
	srv := NewServer(":0", "k", mock)
	r := srv.GetRouter()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)
	if rr.Header().Get("Content-Encoding") == "gzip" {
		t.Fatalf("unexpected gzip without Accept-Encoding")
	}
}
