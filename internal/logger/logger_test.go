package logger

import (
	"github.com/Heidric/metrics.git/pkg/log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_NoPanic(t *testing.T) {
	cfg := &log.Config{Level: "info"}
	_, err := Initialize(cfg)
	if err != nil {
		t.Fatalf("init logger: %v", err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	h := Middleware(next)
	req := httptest.NewRequest("GET", "http://example/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatalf("want 204, got %d", w.Code)
	}
}
