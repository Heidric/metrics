package services

import (
	"context"
	"strconv"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/errors"
	"github.com/Heidric/metrics.git/internal/model"
)

type MetricsService struct {
	storage db.MetricsStorage
}

func NewMetricsService(storage db.MetricsStorage) *MetricsService {
	return &MetricsService{storage: storage}
}

func (m *MetricsService) ListMetrics() map[string]string {
	result := make(map[string]string)
	gauges, counters, err := m.storage.GetAll()
	if err != nil {
		return result
	}

	for name, value := range gauges {
		result[name] = strconv.FormatFloat(value, 'f', -1, 64)
	}
	for name, delta := range counters {
		result[name] = strconv.FormatInt(delta, 10)
	}
	return result
}

func (m *MetricsService) GetMetric(metricType, metricName string) (string, error) {
	switch metricType {
	case "gauge":
		val, err := m.storage.GetGauge(metricName)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	case "counter":
		val, err := m.storage.GetCounter(metricName)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(val, 10), nil
	default:
		return "", errors.ErrInvalidType
	}
}

func (m *MetricsService) UpdateGauge(name, value string) error {
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return errors.ErrInvalidValue
	}
	return m.storage.SetGauge(name, val)
}

func (m *MetricsService) UpdateCounter(name, value string) error {
	delta, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return errors.ErrInvalidValue
	}
	return m.storage.SetCounter(name, delta)
}

func (m *MetricsService) UpdateMetricJSON(metric *model.Metrics) error {
	if metric == nil {
		return errors.ErrInvalidValue
	}

	switch metric.MType {
	case "gauge":
		if metric.Value == nil {
			return errors.ErrInvalidValue
		}
		return m.storage.SetGauge(metric.ID, *metric.Value)
	case "counter":
		if metric.Delta == nil {
			return errors.ErrInvalidValue
		}
		return m.storage.SetCounter(metric.ID, *metric.Delta)
	default:
		return errors.ErrInvalidType
	}
}

func (m *MetricsService) GetMetricJSON(metric *model.Metrics) error {
	if metric == nil {
		return errors.ErrInvalidValue
	}

	switch metric.MType {
	case "gauge":
		value, err := m.storage.GetGauge(metric.ID)
		if err != nil {
			return err
		}
		metric.Value = &value
		metric.Delta = nil
		return nil
	case "counter":
		delta, err := m.storage.GetCounter(metric.ID)
		if err != nil {
			return err
		}
		metric.Delta = &delta
		metric.Value = nil
		return nil
	default:
		return errors.ErrInvalidType
	}
}

func (m *MetricsService) UpdateMetricsBatch(metrics []*model.Metrics) error {
	var valid []*model.Metrics
	for _, metric := range metrics {
		if metric == nil || metric.ID == "" || metric.MType == "" {
			continue
		}
		switch metric.MType {
		case "gauge":
			if metric.Value == nil {
				continue
			}
		case "counter":
			if metric.Delta == nil {
				continue
			}
		default:
			continue
		}
		valid = append(valid, metric)
	}
	if len(valid) == 0 {
		return nil
	}
	return m.storage.UpdateMetricsBatch(valid)
}

func (m *MetricsService) Ping(ctx context.Context) error {
	return m.storage.Ping(ctx)
}
