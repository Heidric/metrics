package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/server"
	"github.com/Heidric/metrics.git/internal/services"
)

func TestServerEndpoints(t *testing.T) {
	storage := db.NewStore()
	defer storage.Close()
	service := services.NewMetricsService(storage)
	srv := server.NewServer(":8080", service)
	testServer := httptest.NewServer(srv.Srv.Handler)
	defer testServer.Close()

	tests := []struct {
		name       string
		method     string
		url        string
		wantStatus int
	}{
		{
			name:       "List metrics",
			method:     "GET",
			url:        "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "Update gauge",
			method:     "POST",
			url:        "/update/gauge/temp/42.5",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, testServer.URL+tt.url, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

func TestGetServerAddress(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantAddr string
		wantErr  bool
	}{
		{
			name:     "Default address",
			args:     []string{},
			wantAddr: "localhost:8080",
			wantErr:  false,
		},
		{
			name:     "Custom address",
			args:     []string{"-a=127.0.0.1:9090"},
			wantAddr: "127.0.0.1:9090",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			oldFlagCommandLine := flag.CommandLine
			defer func() {
				os.Args = oldArgs
				flag.CommandLine = oldFlagCommandLine
			}()

			os.Args = append([]string{"cmd"}, tt.args...)
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

			var buf strings.Builder
			log.SetOutput(&buf)
			defer log.SetOutput(os.Stderr)

			addr := getServerAddress()

			if addr != tt.wantAddr {
				t.Errorf("Expected address %q, got %q", tt.wantAddr, addr)
			}
		})
	}
}
