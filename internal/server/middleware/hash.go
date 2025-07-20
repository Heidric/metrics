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

func (w *hashResponseWriter) WriteHeader(code int) {
	w.wroteHeader = true
}

func (w *hashResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}
