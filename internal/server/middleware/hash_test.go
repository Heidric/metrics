package middleware

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Heidric/metrics.git/internal/crypto"
)

func TestHashMiddleware_PassThroughWhenKeyEmpty(t *testing.T) {
	mw := HashMiddleware("")
	body := []byte("hello")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write(body)
	})
	h := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected %d, got %d", http.StatusTeapot, rec.Code)
	}
	if got := rec.Header().Get("HashSHA256"); got != "" {
		t.Fatalf("expected empty hash header, got %q", got)
	}
	if rec.Body.String() != string(body) {
		t.Fatalf("expected body %q, got %q", body, rec.Body.String())
	}
}

func TestHashMiddleware_ComputesHashAndWritesOKIfNoHeader(t *testing.T) {
	key := "sekret"
	mw := HashMiddleware(key)
	body := []byte("payload to be hashed")

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	})
	h := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	expected := crypto.HashSHA256(body, key)
	got := rec.Header().Get("HashSHA256")
	if got == "" {
		t.Fatalf("expected HashSHA256 header to be set")
	}

	if _, err := hex.DecodeString(got); err != nil {
		t.Fatalf("invalid hex in HashSHA256 header: %v", err)
	}
	if got != expected {
		t.Fatalf("unexpected hash: want %q got %q", expected, got)
	}

	if rec.Body.String() != string(body) {
		t.Fatalf("body mismatch: want %q got %q", body, rec.Body.String())
	}
}
