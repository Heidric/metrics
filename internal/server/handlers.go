package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Heidric/metrics.git/internal/customerrors"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/model"
	"github.com/go-chi/chi"
)

// getMetricHandler returns a metric value addressed by path parameters.
// Route: GET /value/{type}/{name}.
// Content: text/plain.
// Success: 200 OK with text/plain body.
// Errors: 404 Not Found — metric key not found; 400 Bad Request — invalid metric type.
func (s *Server) getMetricHandler(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "metricType")
	metricName := chi.URLParam(r, "metricName")

	metric, err := s.metrics.GetMetric(metricType, metricName)
	if err != nil {
		if err == customerrors.ErrKeyNotFound {
			customerrors.WriteError(w, http.StatusNotFound, "")
			return
		}
		logger.Log.Error().Msgf("Failed to get metric [%s]: %v", metricName, err)
		customerrors.WriteError(w, http.StatusInternalServerError, "")
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, metric)
}

// updateMetricHandler updates a metric addressed by path parameters.
// Route: POST /update/{type}/{name}/{value}.
// Success: 200 OK on success.
// Errors: 400 Bad Request — invalid value or type; 500 Internal Server Error — storage failure.
func (s *Server) updateMetricHandler(w http.ResponseWriter, r *http.Request) {
	metricType := chi.URLParam(r, "metricType")
	name := chi.URLParam(r, "metricName")
	value := chi.URLParam(r, "metricValue")

	if strings.TrimSpace(name) == "" {
		customerrors.WriteError(w, http.StatusNotFound, "")
		return
	}

	var err error
	switch metricType {
	case model.GaugeType:
		err = s.metrics.UpdateGauge(name, value)
	case model.CounterType:
		err = s.metrics.UpdateCounter(name, value)
	default:
		customerrors.WriteError(w, http.StatusBadRequest, "Invalid metric type")
		return
	}

	if err != nil {
		if err == customerrors.ErrInvalidValue {
			customerrors.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		logger.Log.Error().Msgf("Failed to update metric [%s]: %v", name, err)
		customerrors.WriteError(w, http.StatusInternalServerError, "")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// listMetricsHandler renders an HTML overview of all metrics.
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

// notFoundHandler handles requests to the routes that don't exist
func (s *Server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	customerrors.WriteError(w, http.StatusNotFound, "")
}

// updateMetricJSONHandler updates a metric from a JSON payload.
// Route: POST /update.
// Content: application/json request; text/plain or JSON response.
// Success: 200 OK on success.
// Errors: 400 Bad Request — invalid JSON; 400 Bad Request — invalid metric type or value; 500 Internal Server Error — storage failure.
func (s *Server) updateMetricJSONHandler(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var metric model.Metrics
	if err := json.Unmarshal(bodyBytes, &metric); err != nil {
		customerrors.WriteError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	if err := s.metrics.UpdateMetricJSON(&metric); err != nil {
		switch {
		case errors.Is(err, customerrors.ErrInvalidType),
			errors.Is(err, customerrors.ErrInvalidValue):
			customerrors.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, customerrors.ErrKeyNotFound):
			customerrors.WriteError(w, http.StatusNotFound, "")
		default:
			logger.Log.Error().Msgf("Failed to update metric [%s]: %v", metric.MType, err)
			customerrors.WriteError(w, http.StatusInternalServerError, "")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metric)
}

// getMetricJSONHandler returns a metric value based on a JSON payload request.
// Route: POST /value.
// Content: application/json request and response.
// Success: 200 OK with JSON body on success.
// Errors: 400 Bad Request — invalid JSON or metric type; 404 Not Found — metric key not found.
func (s *Server) getMetricJSONHandler(w http.ResponseWriter, r *http.Request) {
	var metric model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		customerrors.WriteError(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	if err := s.metrics.GetMetricJSON(&metric); err != nil {
		switch {
		case errors.Is(err, customerrors.ErrInvalidType):
			customerrors.WriteError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, customerrors.ErrKeyNotFound):
			customerrors.WriteError(w, http.StatusNotFound, "")
		default:
			logger.Log.Error().Msgf("Failed to get metric [%s]: %v", metric.MType, err)
			customerrors.WriteError(w, http.StatusInternalServerError, "")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metric)
}

// updateMetricsBatchHandler applies multiple metric updates in one request.
// Route: POST /updates.
// Content: application/json (array of metrics).
// Success: 200 OK on success.
// Errors: 400 Bad Request — invalid JSON or metrics array; 500 Internal Server Error — storage failure.
func (s *Server) updateMetricsBatchHandler(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var metrics []*model.Metrics
	if err := json.Unmarshal(bodyBytes, &metrics); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	if err := s.metrics.UpdateMetricsBatch(metrics); err != nil {
		http.Error(w, "Batch update failed", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// pingHandler performs a health check against the storage backend.
// Route: GET /ping.
// Content: text/plain.
// Success: 200 OK when storage is healthy.
// Errors: 500 Internal Server Error — storage not reachable.
func (s *Server) pingHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.metrics.Ping(r.Context()); err != nil {
		logger.Log.Error().Msgf("Ping failed: %v", err)
		customerrors.WriteError(w, http.StatusInternalServerError, "")
		return
	}
	w.WriteHeader(http.StatusOK)
}
