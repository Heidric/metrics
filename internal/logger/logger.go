package logger

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/Heidric/metrics.git/pkg/log"
)

// Log is the process-wide zerolog logger used by the service.
var Log *zerolog.Logger

// Initialize configures the logging subsystem using the provided Config.
// It builds a zerolog-based logger (level/format taken from Config), assigns
// the global Log, and returns a wrapper for further use.
func Initialize(config *log.Config) (*log.Logger, error) {
	logger, err := log.NewLogger(context.Background(), config)
	if err != nil {
		return nil, errors.Wrap(err, "new logger")
	}

	Log = logger.Zerolog()

	return logger, nil
}

// Middleware is an HTTP logging middleware.
// It wraps the ResponseWriter to record status and bytes written, measures
// request duration, and logs method, path, status, size, and latency.
// The output is written via the global Log.
func Middleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		responseData := &responseData{
			status: 0,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}
		next.ServeHTTP(&lw, r)

		Log.Info().
			Str("uri", r.RequestURI).
			Str("method", r.Method).
			Str("duration", time.Since(start).String()).
			Int("status", responseData.status).
			Int("size", responseData.size).
			Msg("got HTTP request")
	}

	return http.HandlerFunc(fn)
}

type responseData struct {
	status int
	size   int
}

type loggingResponseWriter struct {
	http.ResponseWriter
	responseData *responseData
}

// Write implements http.ResponseWriter for the logging wrapper.
// It forwards bytes to the underlying writer and accumulates the number
// of bytes written for inclusion in the access log.
func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

// WriteHeader implements http.ResponseWriter for the logging wrapper.
// It forwards the status code to the underlying writer and stores it
// so the middleware can log the final response status.
func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}
