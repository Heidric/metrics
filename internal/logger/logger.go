package logger

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/Heidric/metrics.git/pkg/log"
)

var Log *zerolog.Logger

func Initialize(config *log.Config) (*log.Logger, error) {
	logger, err := log.NewLogger(context.Background(), config)
	if err != nil {
		return nil, errors.Wrap(err, "new logger")
	}

	Log = logger.Zerolog()

	return logger, nil
}

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

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}
