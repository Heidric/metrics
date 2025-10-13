package services

import (
	"context"
	"strconv"

	"github.com/Heidric/metrics.git/internal/customerrors"
	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/model"
)

type MetricsService struct {
	storage db.MetricsStorage
}

func NewMetricsService(storage db.MetricsStorage) *MetricsService {
	return &MetricsService{storage: storage}
}

func (m *MetricsService) ListMetrics() map[string]string {
	ctx := context.Background()
	result := make(map[string]string)
	gauges, counters, err := m.storage.GetAll(ctx)
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
	ctx := context.Background()
	switch metricType {
	case model.GaugeType:
		val, err := m.storage.GetGauge(ctx, metricName)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	case model.CounterType:
		val, err := m.storage.GetCounter(ctx, metricName)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(val, 10), nil
	default:
		return "", customerrors.ErrInvalidType
	}
}

func (m *MetricsService) UpdateGauge(name, value string) error {
	ctx := context.Background()
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return customerrors.ErrInvalidValue
	}
	return m.storage.SetGauge(ctx, name, val)
}

func (m *MetricsService) UpdateCounter(name, value string) error {
	ctx := context.Background()
	delta, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return customerrors.ErrInvalidValue
	}
	return m.storage.SetCounter(ctx, name, delta)
}

func (m *MetricsService) UpdateMetricJSON(metric *model.Metrics) error {
	ctx := context.Background()
	if metric == nil {
		return customerrors.ErrInvalidValue
	}

	switch metric.MType {
	case model.GaugeType:
		if metric.Value == nil {
			return customerrors.ErrInvalidValue
		}
		return m.storage.SetGauge(ctx, metric.ID, *metric.Value)
	case model.CounterType:
		if metric.Delta == nil {
			return customerrors.ErrInvalidValue
		}
		return m.storage.SetCounter(ctx, metric.ID, *metric.Delta)
	default:
		return customerrors.ErrInvalidType
	}
}

func (m *MetricsService) GetMetricJSON(metric *model.Metrics) error {
	ctx := context.Background()
	if metric == nil {
		return customerrors.ErrInvalidValue
	}

	switch metric.MType {
	case model.GaugeType:
		value, err := m.storage.GetGauge(ctx, metric.ID)
		if err != nil {
			return err
		}
		metric.Value = &value
		metric.Delta = nil
		return nil
	case model.CounterType:
		delta, err := m.storage.GetCounter(ctx, metric.ID)
		if err != nil {
			return err
		}
		metric.Delta = &delta
		metric.Value = nil
		return nil
	default:
		return customerrors.ErrInvalidType
	}
}

func (m *MetricsService) UpdateMetricsBatch(metrics []*model.Metrics) error {
	ctx := context.Background()
	valid := metrics[:0]
	for _, metric := range metrics {
		if metric == nil || metric.ID == "" || metric.MType == "" {
			continue
		}
		switch metric.MType {
		case model.GaugeType:
			if metric.Value == nil {
				continue
			}
		case model.CounterType:
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
	return m.storage.UpdateMetricsBatch(ctx, valid)
}

func (m *MetricsService) Ping(ctx context.Context) error {
	return m.storage.Ping(ctx)
}
