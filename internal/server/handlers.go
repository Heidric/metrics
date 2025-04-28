package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Heidric/metrics.git/internal/errors"
	"github.com/go-chi/chi"
)

func (s *Server) getMetricHandler(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "metricType")
	metricName := chi.URLParam(r, "metricName")

	metric, err := s.metrics.GetMetric(metricType, metricName)
	if err != nil {
		if err == errors.ErrKeyNotFound {
			errors.NotFoundError(w)
			return
		}
		errors.InternalError(w)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, metric)
}

func (s *Server) updateMetricHandler(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "metricType")
	name := chi.URLParam(r, "metricName")
	value := chi.URLParam(r, "metricValue")

	if strings.TrimSpace(name) == "" {
		errors.NotFoundError(w)
		return
	}

	var err error
	switch metricType {
	case "gauge":
		err = s.metrics.UpdateGauge(name, value)
	case "counter":
		err = s.metrics.UpdateCounter(name, value)
	default:
		errors.ValidationError(w, "Invalid metric type")
		return
	}

	if err != nil {
		if err == errors.ErrInvalidValue {
			errors.ValidationError(w, err.Error())
			return
		}
		errors.InternalError(w)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) listMetricsHandler(w http.ResponseWriter, r *http.Request) {
	allMetrics := s.metrics.ListMetrics()

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	html := `<html><head><title>Metrics List</title></head><body>
             <h1>Metrics</h1>
             <table border="1">
             <tr><th>Name</th><th>Value</th></tr>`

	for name, value := range allMetrics {
		html += fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", name, value)
	}

	html += "</table></body></html>"
	w.Write([]byte(html))
}

func (s *Server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	errors.NotFoundError(w)
}
