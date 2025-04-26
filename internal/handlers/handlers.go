package handlers

import (
	"net/http"
	"strings"

	"github.com/Heidric/metrics.git/internal/errors"
	"github.com/Heidric/metrics.git/internal/services"
)

type MetricsHandlers struct {
	service *services.MetricsService
}

func NewMetricsHandlers(service *services.MetricsService) *MetricsHandlers {
	return &MetricsHandlers{service: service}
}

func (h *MetricsHandlers) UpdateMetricHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	parts := strings.Split(path, "/")
	var metric, name, value string
	if len(parts) == 5 {
		metric, name, value = parts[2], parts[3], parts[4]
	}
	if strings.TrimSpace(name) == "" {
		errors.NotFoundError(w)
		return
	}

	var err error
	switch metric {
	case "gauge":
		err = h.service.UpdateGauge(name, value)
	case "counter":
		err = h.service.UpdateCounter(name, value)
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

func (h *MetricsHandlers) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	errors.NotFoundError(w)
}
