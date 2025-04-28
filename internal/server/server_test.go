package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/services"
)

func TestServerRoutes(t *testing.T) {
	storage := db.NewStore()
	defer storage.Close()
	service := services.NewMetricsService(storage)

	srv := NewServer(":8080", service)
	testServer := httptest.NewServer(srv.Srv.Handler)
	defer testServer.Close()

	t.Run("Update gauge", func(t *testing.T) {
		req, err := http.NewRequest("POST", testServer.URL+"/update/gauge/temp/42.5", nil)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("List metrics", func(t *testing.T) {
		req, err := http.NewRequest("GET", testServer.URL+"/", nil)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		if ct := resp.Header.Get("Content-Type"); ct != "text/html" {
			t.Errorf("Expected Content-Type text/html, got %s", ct)
		}
	})
}
