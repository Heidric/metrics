package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/handlers"
	"github.com/Heidric/metrics.git/internal/services"
)

func TestServer(t *testing.T) {
	storage := db.NewKeyValueStore()
	defer storage.Close()
	service := services.NewMetricsService(storage)
	handler := handlers.NewMetricsHandlers(service)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/update/") {
			handler.UpdateMetricHandler(w, r)
		} else {
			handler.NotFoundHandler(w, r)
		}
	}))
	defer server.Close()

	t.Run("Successful gauge update", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/update/gauge/temp/42", "", nil)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				return
			}
		}(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		value, err := storage.Get("temp")
		if err != nil {
			t.Fatalf("Failed to get value: %v", err)
		}
		if value != "42" {
			t.Fatalf("Expected '42', got '%s'", value)
		}
	})

	t.Run("Successful counter update", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/update/counter/hits/10", "", nil)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				return
			}
		}(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		value, err := storage.Get("hits")
		if err != nil {
			t.Fatalf("Failed to get value: %v", err)
		}
		if value != "10" {
			t.Fatalf("Expected '10', got '%s'", value)
		}

		resp, err = http.Post(server.URL+"/update/counter/hits/5", "", nil)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				return
			}
		}(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		value, err = storage.Get("hits")
		if err != nil {
			t.Fatalf("Failed to get value: %v", err)
		}
		if value != "15" {
			t.Fatalf("Expected '15', got '%s'", value)
		}
	})

	t.Run("Invalid path", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/update/invalid/temp/42", "", nil)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				return
			}
		}(resp.Body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("Not found", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/nonexistent")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				return
			}
		}(resp.Body)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("Expected status 404, got %d", resp.StatusCode)
		}
	})
}
