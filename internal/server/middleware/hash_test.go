package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Heidric/metrics.git/internal/crypto"
)

func getHashHeader(h http.Header) string {
	if v := h.Get("HashSHA256"); v != "" {
		return v
	}
	return h.Get("Hash")
}

func TestHashMiddleware_ComputesHashAndWritesBody(t *testing.T) {
	hashKey := "test-key"
	mw := HashMiddleware(hashKey)

	payload := []byte("payload-data")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	})

	rr := httptest.NewRecorder()
	wrapped := mw(next)
	wrapped.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Body.Bytes(); string(got) != string(payload) {
		t.Fatalf("body = %q, want %q", string(got), string(payload))
	}
	exp := crypto.HashSHA256(payload, hashKey)
	if hdr := getHashHeader(rr.Header()); hdr != exp {
		t.Fatalf("hash header = %q, want %q", hdr, exp)
	}
}

func TestHashMiddleware_RespectsExplicitStatus(t *testing.T) {
	hashKey := "k"
	mw := HashMiddleware(hashKey)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("x"))
	})

	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	if rr.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusTeapot)
	}
	exp := crypto.HashSHA256([]byte("x"), hashKey)
	if hdr := getHashHeader(rr.Header()); hdr != exp {
		t.Fatalf("hash header = %q, want %q", hdr, exp)
	}
}

func TestHashMiddleware_EmptyResponse_DefaultsTo200AndHashesEmpty(t *testing.T) {
	hashKey := "empty"
	mw := HashMiddleware(hashKey)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("body len = %d, want 0", rr.Body.Len())
	}
	exp := crypto.HashSHA256(nil, hashKey) // hash of empty body
	if hdr := getHashHeader(rr.Header()); hdr != exp {
		t.Fatalf("hash header = %q, want %q", hdr, exp)
	}
}

func TestHashMiddleware_MultiWrites_ComputesHashOverWholeBody(t *testing.T) {
	hashKey := "multi"
	mw := HashMiddleware(hashKey)

	chunks := [][]byte{
		[]byte("alpha"),
		[]byte(""),
		[]byte("beta"),
		[]byte("γ"),
		[]byte("delta"),
	}
	full := bytes.Join(chunks, nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, c := range chunks {
			if _, err := w.Write(c); err != nil {
				t.Fatalf("write failed: %v", err)
			}
		}
	})

	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !bytes.Equal(rr.Body.Bytes(), full) {
		t.Fatalf("body mismatch:\n got: %q\nwant: %q", rr.Body.Bytes(), full)
	}
	exp := crypto.HashSHA256(full, hashKey)
	if hdr := getHashHeader(rr.Header()); hdr != exp {
		t.Fatalf("hash header = %q, want %q", hdr, exp)
	}
}
