package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/services"
	"github.com/rs/zerolog"
)

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
		wantStatus    int
		wantHeader    string
		wantHeaderVal string
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
