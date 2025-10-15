package logger

import (
	"net/http/httptest"
	"testing"

	"github.com/Heidric/metrics.git/pkg/log"
)

func TestInitialize_DefaultConfig(t *testing.T) {
	cfg := &log.Config{}
	lg, err := Initialize(cfg)
	if err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	if lg == nil {
		t.Fatalf("Initialize returned nil logger")
	}
}

func TestLoggingResponseWriter_Write_AccumulatesSizeAndWrites(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &loggingResponseWriter{
		ResponseWriter: rr,
		responseData:   &responseData{},
	}
	payload := []byte("hello world")
	n, err := rw.Write(payload)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("Write n=%d, want %d", n, len(payload))
	}
	if got := rw.responseData.size; got != len(payload) {
		t.Fatalf("size=%d, want %d", got, len(payload))
	}
	if body := rr.Body.String(); body != string(payload) {
		t.Fatalf("body=%q, want %q", body, payload)
	}
}

func TestLoggingResponseWriter_WriteHeader_SetsStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &loggingResponseWriter{
		ResponseWriter: rr,
		responseData:   &responseData{},
	}

	rw.WriteHeader(418)
	if rr.Code != 418 {
		t.Fatalf("status=%d, want 418", rr.Code)
	}
	if rw.responseData.status != 418 {
		t.Fatalf("responseData.status=%d, want 418", rw.responseData.status)
	}
}
