package services

import (
	"context"
	"errors"
	"testing"

	"github.com/Heidric/metrics.git/internal/customerrors"
	"github.com/Heidric/metrics.git/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorage struct {
	gauges               map[string]float64
	counters             map[string]int64
	updateMetricsBatchFn func(metrics []*model.Metrics) error
}

func floatPtr(f float64) *float64 { return &f }
func intPtr(i int64) *int64       { return &i }

func (m *mockStorage) SetGauge(ctx context.Context, name string, value float64) error {
	m.gauges[name] = value
	return nil
}

func (m *mockStorage) GetGauge(ctx context.Context, name string) (float64, error) {
	val, ok := m.gauges[name]
	if !ok {
		return 0, customerrors.ErrKeyNotFound
	}
	return val, nil
}

func (m *mockStorage) SetCounter(ctx context.Context, name string, value int64) error {
	if m.counters == nil {
		m.counters = make(map[string]int64)
	}

	current := m.counters[name]
	m.counters[name] = current + value
	return nil
}

func (m *mockStorage) GetCounter(ctx context.Context, name string) (int64, error) {
	val, ok := m.counters[name]
	if !ok {
		return 0, customerrors.ErrKeyNotFound
	}
	return val, nil
}

func (m *mockStorage) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	return m.gauges, m.counters, nil
}

func (m *mockStorage) UpdateMetricsBatch(ctx context.Context, metrics []*model.Metrics) error {
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

		val, err := service.GetMetric(model.GaugeType, "temp")
		require.NoError(t, err)
		assert.Equal(t, "42.5", val)
	})

	t.Run("GetMetric counter", func(t *testing.T) {
		storage := &mockStorage{
			gauges:   make(map[string]float64),
			counters: map[string]int64{"hits": 15},
		}
		service := NewMetricsService(storage)

		val, err := service.GetMetric(model.CounterType, "hits")
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

func TestUpdateGauge_InvalidValue_ReturnsErrInvalidValue(t *testing.T) {
	storage := &mockStorage{gauges: map[string]float64{}}
	service := NewMetricsService(storage)

	err := service.UpdateGauge("g", "not-a-float")
	assert.ErrorIs(t, err, customerrors.ErrInvalidValue)
	_, ok := storage.gauges["g"]
	assert.False(t, ok, "storage must not be updated on invalid input")
}

func TestUpdateCounter_InvalidValue_ReturnsErrInvalidValue(t *testing.T) {
	storage := &mockStorage{counters: map[string]int64{}}
	service := NewMetricsService(storage)

	err := service.UpdateCounter("c", "not-an-int")
	assert.ErrorIs(t, err, customerrors.ErrInvalidValue)
	_, ok := storage.counters["c"]
	assert.False(t, ok, "storage must not be updated on invalid input")
}

func TestUpdateMetricJSON_InvalidType(t *testing.T) {
	storage := &mockStorage{}
	service := NewMetricsService(storage)

	m := &model.Metrics{ID: "x", MType: "weird"}
	err := service.UpdateMetricJSON(m)
	assert.ErrorIs(t, err, customerrors.ErrInvalidType)
}

func TestGetMetricJSON_InvalidType(t *testing.T) {
	storage := &mockStorage{}
	service := NewMetricsService(storage)

	m := &model.Metrics{ID: "x", MType: "nope"}
	err := service.GetMetricJSON(m)
	assert.ErrorIs(t, err, customerrors.ErrInvalidType)
}

func TestUpdateMetricsBatch_FiltersInvalidAndCallsStorage(t *testing.T) {
	called := 0
	var got []*model.Metrics
	storage := &mockStorage{
		updateMetricsBatchFn: func(metrics []*model.Metrics) error {
			called++
			got = metrics
			return nil
		},
	}
	service := NewMetricsService(storage)

	in := []*model.Metrics{
		{ID: "a", MType: model.GaugeType, Value: floatPtr(1)},
		{ID: "b", MType: "unknown"},
		{ID: "c", MType: model.CounterType, Delta: intPtr(5)},
	}

	err := service.UpdateMetricsBatch(in)
	assert.NoError(t, err)
	assert.Equal(t, 1, called, "storage should be called exactly once")
	assert.Len(t, got, 2, "only two valid metrics must pass through")
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, "c", got[1].ID)
}

func TestUpdateMetricsBatch_AllInvalid_NoCall(t *testing.T) {
	called := 0
	storage := &mockStorage{
		updateMetricsBatchFn: func(metrics []*model.Metrics) error {
			called++
			return errors.New("should not be called")
		},
	}
	service := NewMetricsService(storage)

	in := []*model.Metrics{
		{ID: "a", MType: "nope"},
		{ID: "b", MType: "also-nope"},
	}

	err := service.UpdateMetricsBatch(in)
	assert.NoError(t, err)
	assert.Equal(t, 0, called, "storage must not be called when all invalid")
}
