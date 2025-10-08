package services

import (
	"context"
	"strconv"

	"github.com/Heidric/metrics.git/internal/customerrors"
	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/model"
)

// MetricsService orchestrates domain-level operations on metrics.
// It validates/coerces inputs (e.g., parses numeric strings), enforces
// gauge/counter semantics, and delegates persistence to db.MetricsStorage.
type MetricsService struct {
	storage db.MetricsStorage
}

// NewMetricsService constructs a metrics service backed by the provided storage.
func NewMetricsService(storage db.MetricsStorage) *MetricsService {
	return &MetricsService{storage: storage}
}

// ListMetrics returns a snapshot map of metric names to their stringified values.
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

// GetMetric retrieves a metric by type and name and returns its string value.
// Errors:
//   - customerrors.ErrInvalidType — unsupported type
//   - customerrors.ErrKeyNotFound — metric key does not exist
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

// UpdateGauge sets a gauge metric to the provided numeric value.
// The value is parsed as float64
// Errors:
//   - customerrors.ErrInvalidValue — cannot parse
//   - underlying storage error
func (m *MetricsService) UpdateGauge(name, value string) error {
	ctx := context.Background()
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return customerrors.ErrInvalidValue
	}
	return m.storage.SetGauge(ctx, name, val)
}

// UpdateCounter adds the provided delta to a counter metric.
// The delta is parsed as int64; if the metric does not exist, it is created.
// Errors:
//   - customerrors.ErrInvalidValue — cannot parse delta
//   - underlying storage error
func (m *MetricsService) UpdateCounter(name, value string) error {
	ctx := context.Background()
	delta, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return customerrors.ErrInvalidValue
	}
	return m.storage.SetCounter(ctx, name, delta)
}

// UpdateMetricJSON updates a metric from a JSON payload.
// Gauge requires Value to be set; counter requires Delta to be set.
// Errors:
//   - customerrors.ErrInvalidType — unsupported type
//   - customerrors.ErrInvalidValue — missing/wrong field for the type
//   - underlying storage error
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

// GetMetricJSON populates the given JSON payload with the current metric value.
// Expects ID and MType to be set in the input. Returns ErrNotFound if missing.
// Errors:
//   - customerrors.ErrInvalidType — unsupported type
//   - customerrors.ErrKeyNotFound — metric key does not exist
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

// UpdateMetricsBatch applies multiple metric updates in one request.
// Invalid items are skipped; valid ones are forwarded to storage in batch.
// Returns nil if the input contains no valid items.
// Errors:
//   - underlying storage error for the batch write
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

// Ping performs a health check against the underlying storage backend.
func (m *MetricsService) Ping(ctx context.Context) error {
	return m.storage.Ping(ctx)
}
