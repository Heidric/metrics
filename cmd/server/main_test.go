package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/server"
	"github.com/Heidric/metrics.git/internal/services"
)

func TestServer(t *testing.T) {
	storage := db.NewStore()
	defer storage.Close()
	service := services.NewMetricsService(storage)
	srv := server.NewServer(":8080", service)
	testServer := httptest.NewServer(srv.Srv.Handler)
	defer testServer.Close()

	t.Run("List metrics returns HTML", func(t *testing.T) {
		// Setup test data
		storage.Set("gauge_temp", "42.5")
		storage.Set("counter_hits", "10")

		req, err := http.NewRequest("GET", testServer.URL+"/", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/html" {
			t.Fatalf("Expected Content-Type text/html, got %s", contentType)
		}
	})

	t.Run("Update and get gauge", func(t *testing.T) {
		req, err := http.NewRequest("POST", testServer.URL+"/update/gauge/temp/42.5", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		// Verify the value
		req, err = http.NewRequest("GET", testServer.URL+"/value/gauge/temp", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Update and get counter", func(t *testing.T) {
		req, err := http.NewRequest("POST", testServer.URL+"/update/counter/hits/10", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		// Verify the value
		req, err = http.NewRequest("GET", testServer.URL+"/value/counter/hits", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}
