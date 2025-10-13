package middleware

import (
	"bytes"
	"net/http"

	"github.com/Heidric/metrics.git/internal/crypto"
)

type hashResponseWriter struct {
	http.ResponseWriter
	buf         *bytes.Buffer
	hashKey     string
	wroteHeader bool
}

// HashMiddleware computes an HMAC-SHA256 of the response body and adds it to
// the "HashSHA256" response header as a hex string.
func HashMiddleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			buf := new(bytes.Buffer)
			hrw := &hashResponseWriter{
				ResponseWriter: w,
				buf:            buf,
				hashKey:        key,
			}

			next.ServeHTTP(hrw, r)

			hash := crypto.HashSHA256(hrw.buf.Bytes(), key)
			w.Header().Set("HashSHA256", hash)

			if !hrw.wroteHeader {
				w.WriteHeader(http.StatusOK)
			}

			w.Write(hrw.buf.Bytes())
		})
	}
}

// WriteHeader defers sending the status code until the middleware finishes
// computing the hash and writes the buffered body.
func (w *hashResponseWriter) WriteHeader(code int) {
	w.wroteHeader = true
}

// Write appends bytes to the internal buffer; the real write happens after
// the hash is computed in the middleware.
func (w *hashResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}
