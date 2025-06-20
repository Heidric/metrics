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
	"github.com/go-chi/chi"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

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

type Server struct {
	Srv     *http.Server
	metrics Metrics
	logger  *zerolog.Logger
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (g gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

func NewServer(addr string, metrics Metrics) *Server {
	logger := zerolog.Nop()

	r := chi.NewRouter()
	s := &Server{
		Srv:     &http.Server{Addr: addr, Handler: r},
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
		r.Post("/value/", s.getMetricJSONHandler)
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

func (s *Server) Run(ctx context.Context, runner *errgroup.Group) {
	logger.Log.Info().Msg("Http server started.")

	runner.Go(func() error {
		if err := s.Srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})
}

func (s *Server) Shutdown(ctx context.Context) error {
	logger.Log.Info().Msg("Http server stopped.")

	nctx, stop := context.WithTimeout(ctx, time.Second*10)
	defer stop()

	return s.Srv.Shutdown(nctx)
}

func (s *Server) GetRouter() *chi.Mux {
	return s.Srv.Handler.(*chi.Mux)
}
