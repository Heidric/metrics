package customerrors

import (
	"net/http"
	"testing"
)

type noopWriter struct {
	code   int
	header http.Header
}

func (w *noopWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *noopWriter) Write(b []byte) (int, error) { return len(b), nil }

func (w *noopWriter) WriteHeader(statusCode int) { w.code = statusCode }

func BenchmarkWriteError_400(b *testing.B) { benchWrite(b, http.StatusBadRequest) }
func BenchmarkWriteError_404(b *testing.B) { benchWrite(b, http.StatusNotFound) }
func BenchmarkWriteError_500(b *testing.B) { benchWrite(b, http.StatusInternalServerError) }

func benchWrite(b *testing.B, code int) {
	w := &noopWriter{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		WriteError(w, code, "benchmark")
		w.code = 0
	}
}
