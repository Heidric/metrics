package services

import (
	"context"
	"testing"

	"github.com/Heidric/metrics.git/internal/errors"
	"github.com/Heidric/metrics.git/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorage struct {
	gauges               map[string]float64
	counters             map[string]int64
	updateMetricsBatchFn func(metrics []*model.Metrics) error
}

func (m *mockStorage) SetGauge(name string, value float64) error {
	m.gauges[name] = value
	return nil
}

func (m *mockStorage) GetGauge(name string) (float64, error) {
	val, ok := m.gauges[name]
	if !ok {
		return 0, errors.ErrKeyNotFound
	}
	return val, nil
}

func (m *mockStorage) SetCounter(name string, value int64) error {
	if m.counters == nil {
		m.counters = make(map[string]int64)
	}

	current := m.counters[name]
	m.counters[name] = current + value
	return nil
}

func (m *mockStorage) GetCounter(name string) (int64, error) {
	val, ok := m.counters[name]
	if !ok {
		return 0, errors.ErrKeyNotFound
	}
	return val, nil
}

func (m *mockStorage) GetAll() (map[string]float64, map[string]int64, error) {
	return m.gauges, m.counters, nil
}

func (m *mockStorage) UpdateMetricsBatch(metrics []*model.Metrics) error {
	if m.updateMetricsBatchFn != nil {
		return m.updateMetricsBatchFn(metrics)
	}
	return nil
}

func (m *mockStorage) Ping(ctx context.Context) error {
	return nil
}

func (m *mockStorage) Close() error {
	return nil
}

func TestMetricsService(t *testing.T) {
	t.Run("UpdateGauge", func(t *testing.T) {
		storage := &mockStorage{
			gauges:   make(map[string]float64),
			counters: make(map[string]int64),
		}
		service := NewMetricsService(storage)

		err := service.UpdateGauge("temp", "42.5")
		require.NoError(t, err)

		assert.Equal(t, 42.5, storage.gauges["temp"])
	})

	t.Run("UpdateCounter", func(t *testing.T) {
		storage := &mockStorage{
			gauges:   make(map[string]float64),
			counters: make(map[string]int64),
		}
		service := NewMetricsService(storage)

		err := service.UpdateCounter("hits", "10")
		require.NoError(t, err)

		err = service.UpdateCounter("hits", "5")
		require.NoError(t, err)

		assert.Equal(t, int64(15), storage.counters["hits"])
	})

	t.Run("GetMetric gauge", func(t *testing.T) {
		storage := &mockStorage{
			gauges:   map[string]float64{"temp": 42.5},
			counters: make(map[string]int64),
		}
		service := NewMetricsService(storage)

		val, err := service.GetMetric("gauge", "temp")
		require.NoError(t, err)
		assert.Equal(t, "42.5", val)
	})

	t.Run("GetMetric counter", func(t *testing.T) {
		storage := &mockStorage{
			gauges:   make(map[string]float64),
			counters: map[string]int64{"hits": 15},
		}
		service := NewMetricsService(storage)

		val, err := service.GetMetric("counter", "hits")
		require.NoError(t, err)
		assert.Equal(t, "15", val)
	})

	t.Run("ListMetrics", func(t *testing.T) {
		storage := &mockStorage{
			gauges:   map[string]float64{"gauge1": 1.1},
			counters: map[string]int64{"counter1": 10},
		}
		service := NewMetricsService(storage)

		metrics := service.ListMetrics()
		assert.Equal(t, 2, len(metrics))
		assert.Equal(t, "1.1", metrics["gauge1"])
		assert.Equal(t, "10", metrics["counter1"])
	})

	t.Run("Ping", func(t *testing.T) {
		storage := &mockStorage{}
		service := NewMetricsService(storage)

		err := service.Ping(context.Background())
		assert.NoError(t, err)
	})
}
