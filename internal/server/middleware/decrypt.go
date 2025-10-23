package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// DecryptFunc takes an encrypted body and returns plaintext JSON.
type DecryptFunc func(ciphertext []byte) (plaintext []byte, err error)

type decryptOnceKey struct{}

// DecryptJSON applies request-body decryption before the handler.
// On failure, it writes 400 Bad Request.
func DecryptJSON(decrypt DecryptFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if decrypt == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := r.Context().Value(decryptOnceKey{}).(bool); ok {
				next.ServeHTTP(w, r)
				return
			}

			cipher, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			_ = r.Body.Close()
			if len(cipher) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			plain, err := decrypt(cipher)
			if err != nil {
				http.Error(w, "Failed to decrypt body", http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(plain))
			r.ContentLength = int64(len(plain))
			r.Header.Del("Content-Length")
			r = r.WithContext(context.WithValue(r.Context(), decryptOnceKey{}, true))

			next.ServeHTTP(w, r)
		})
	}
}
