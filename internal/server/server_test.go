package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/services"
	"github.com/rs/zerolog"
)

func TestServerRoutes(t *testing.T) {
	// Инициализируем тестовый логгер
	testLogger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.Disabled)
	logger.Log = &testLogger

	storage := db.NewStore()
	defer storage.Close()
	service := services.NewMetricsService(storage)

	srv := NewServer(":8080", service)
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
