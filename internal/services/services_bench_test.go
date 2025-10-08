package services

import (
	"context"
	"strconv"
	"testing"

	"github.com/Heidric/metrics.git/internal/customerrors"
	"github.com/Heidric/metrics.git/internal/model"
)

type benchStore struct {
	g map[string]float64
	c map[string]int64
}

func (m *benchStore) ensure() {
	if m.g == nil {
		m.g = make(map[string]float64, 1024)
	}
	if m.c == nil {
		m.c = make(map[string]int64, 1024)
	}
}

func (m *benchStore) SetGauge(_ context.Context, name string, value float64) error {
	m.ensure()
	m.g[name] = value
	return nil
}

func (m *benchStore) SetCounter(_ context.Context, name string, value int64) error {
	m.ensure()
	m.c[name] += value
	return nil
}

func (m *benchStore) GetGauge(_ context.Context, name string) (float64, error) {
	v, ok := m.g[name]
	if !ok {
		return 0, customerrors.ErrKeyNotFound
	}
	return v, nil
}

func (m *benchStore) GetCounter(_ context.Context, name string) (int64, error) {
	v, ok := m.c[name]
	if !ok {
		return 0, customerrors.ErrKeyNotFound
	}
	return v, nil
}

func (m *benchStore) GetAll(_ context.Context) (map[string]float64, map[string]int64, error) {
	return m.g, m.c, nil
}

func (m *benchStore) UpdateMetricsBatch(ctx context.Context, metrics []*model.Metrics) error {
	for _, mm := range metrics {
		switch mm.MType {
		case model.GaugeType:
			if mm.Value != nil {
				_ = m.SetGauge(ctx, mm.ID, *mm.Value)
			}
		case model.CounterType:
			if mm.Delta != nil {
				_ = m.SetCounter(ctx, mm.ID, *mm.Delta)
			}
		}
	}
	return nil
}

func (m *benchStore) Ping(_ context.Context) error { return nil }

func (m *benchStore) Close() error { return nil }

func BenchmarkUpdateGauge(b *testing.B) {
	svc := NewMetricsService(&benchStore{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = svc.UpdateGauge("g"+strconv.Itoa(i%1024), strconv.Itoa(i%1000))
	}
}

func BenchmarkUpdateCounter(b *testing.B) {
	svc := NewMetricsService(&benchStore{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = svc.UpdateCounter("c"+strconv.Itoa(i%1024), strconv.Itoa(i%10))
	}
}

func BenchmarkUpdateMetricJSON(b *testing.B) {
	svc := NewMetricsService(&benchStore{})
	d := int64(1)
	v := 3.14
	payloads := []*model.Metrics{
		{ID: "cpu", MType: model.GaugeType, Value: &v},
		{ID: "reqs", MType: model.CounterType, Delta: &d},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = svc.UpdateMetricJSON(payloads[i%len(payloads)])
	}
}

func BenchmarkUpdateMetricsBatch_256(b *testing.B)  { benchBatch(b, 256) }
func BenchmarkUpdateMetricsBatch_1024(b *testing.B) { benchBatch(b, 1024) }

func benchBatch(b *testing.B, n int) {
	svc := NewMetricsService(&benchStore{})
	metrics := make([]*model.Metrics, 0, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			v := float64(i)
			metrics = append(metrics, &model.Metrics{ID: "g" + strconv.Itoa(i), MType: model.GaugeType, Value: &v})
		} else {
			d := int64(i)
			metrics = append(metrics, &model.Metrics{ID: "c" + strconv.Itoa(i), MType: model.CounterType, Delta: &d})
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.UpdateMetricsBatch(metrics)
	}
}
