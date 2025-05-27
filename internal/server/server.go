package server

import (
	"context"
	"net/http"
	"time"

	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/go-chi/chi"
	"golang.org/x/sync/errgroup"
)

type Metrics interface {
	ListMetrics() map[string]string
	GetMetric(metricType, metricName string) (string, error)
	UpdateGauge(name, value string) error
	UpdateCounter(name, value string) error
}

type Server struct {
	Srv     *http.Server
	metrics Metrics
}

func NewServer(addr string, metrics Metrics) *Server {
	r := chi.NewRouter()
	s := &Server{
		Srv:     &http.Server{Addr: addr, Handler: r},
		metrics: metrics,
	}

	// Используем middleware из пакета logger
	r.Use(logger.Middleware)

	r.Route("/", func(r chi.Router) {
		r.Get("/", s.listMetricsHandler)
		r.Post("/update/{metricType}/{metricName}/{metricValue}", s.updateMetricHandler)
		r.Get("/value/{metricType}/{metricName}", s.getMetricHandler)
	})

	r.NotFound(s.notFoundHandler)

	return s
}

func (s *Server) Run(ctx context.Context, runner *errgroup.Group) {
	logger.Log.Info().Msg("HTTP server started")

	runner.Go(func() error {
		if err := s.Srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})
}

func (s *Server) Shutdown(ctx context.Context) error {
	logger.Log.Info().Msg("HTTP server stopped")

	nctx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()

	return s.Srv.Shutdown(nctx)
}

func (s *Server) GetRouter() *chi.Mux {
	return s.Srv.Handler.(*chi.Mux)
}
