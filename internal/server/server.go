package server

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/model"
	"github.com/Heidric/metrics.git/internal/server/middleware"
	"github.com/go-chi/chi"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

// Metrics defines the storage contract for metrics: read/update operations
// and health checks. Implement this interface to back the HTTP handlers.
type Metrics interface {
	ListMetrics() map[string]string
	GetMetric(metricType, metricName string) (string, error)
	UpdateGauge(name, value string) error
	UpdateCounter(name, value string) error
	UpdateMetricJSON(metric *model.Metrics) error
	GetMetricJSON(metric *model.Metrics) error
	UpdateMetricsBatch(metrics []*model.Metrics) error
	Ping(ctx context.Context) error
}

// Server wraps the HTTP server and wires routes/middleware for the metrics API.
// Handlers expose read/update operations and a health endpoint. Construct via
// NewServer and run the embedded *http.Server.
type Server struct {
	Srv     *http.Server
	hashKey string
	metrics Metrics
	logger  *zerolog.Logger
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

// Write compresses and writes the response body using the underlying gzip.Writer.
func (g gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

// NewServer configures the router and middleware and returns a ready-to-run
// HTTP server for metrics. The returned Server embeds *http.Server.
//   - addr: listen address (e.g. ":8080")
//   - hashKey: key used by middleware that sign/verify payloads
//   - metrics: storage implementation backing the handlers
func NewServer(addr string, hashKey string, metrics Metrics) *Server {
	logger := zerolog.Nop()

	r := chi.NewRouter()
	s := &Server{
		Srv:     &http.Server{Addr: addr, Handler: r},
		hashKey: hashKey,
		metrics: metrics,
		logger:  &logger,
	}

	r.Use(s.gzipMiddleware)
	r.Use(s.loggingMiddleware)

	r.Route("/", func(r chi.Router) {
		r.Get("/", s.listMetricsHandler)
		r.Post("/update/{metricType}/{metricName}/{metricValue}", s.updateMetricHandler)
		r.Get("/value/{metricType}/{metricName}", s.getMetricHandler)
		r.Post("/update/", s.updateMetricJSONHandler)
		r.With(middleware.HashMiddleware(hashKey)).Post("/value/", s.getMetricJSONHandler)
		r.Post("/updates/", s.updateMetricsBatchHandler)
		r.Get("/ping", s.pingHandler)
	})

	r.NotFound(s.notFoundHandler)

	return s
}

func (s *Server) gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Invalid gzip body", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			r.Body = gz
		}

		acceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
		if !acceptsGzip {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()

		gzWriter := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		next.ServeHTTP(gzWriter, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Dur("duration", time.Since(start)).
			Msg("request processed")
	})
}

// Run starts the HTTP server in the provided errgroup.Group.
// It schedules ListenAndServe on the group and returns immediately.
// http.ErrServerClosed is treated as a normal shutdown signal.
func (s *Server) Run(ctx context.Context, runner *errgroup.Group) {
	logger.Log.Info().Msg("Http server started.")

	runner.Go(func() error {
		if err := s.Srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})
}

// Shutdown performs a graceful shutdown of the HTTP server with a 10-second timeout.
// It stops accepting new connections and waits for in-flight requests to finish.
func (s *Server) Shutdown(ctx context.Context) error {
	logger.Log.Info().Msg("Http server stopped.")

	nctx, stop := context.WithTimeout(ctx, time.Second*10)
	defer stop()

	return s.Srv.Shutdown(nctx)
}

// GetRouter returns the underlying *chi.Mux used as the server's handler.
// Used for tests and for wiring additional routes/middleware at startup.
func (s *Server) GetRouter() *chi.Mux {
	return s.Srv.Handler.(*chi.Mux)
}
