package services

import (
	"context"
	"testing"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/model"
)

type memStore struct {
	g map[string]float64
	c map[string]int64
}

func (m *memStore) SetGauge(ctx context.Context, name string, value float64) error {
	if m.g == nil {
		m.g = map[string]float64{}
	}
	m.g[name] = value
	return nil
}
func (m *memStore) GetGauge(ctx context.Context, name string) (float64, error) { return m.g[name], nil }
func (m *memStore) SetCounter(ctx context.Context, name string, value int64) error {
	if m.c == nil {
		m.c = map[string]int64{}
	}
	m.c[name] += value
	return nil
}
func (m *memStore) GetCounter(ctx context.Context, name string) (int64, error) { return m.c[name], nil }
func (m *memStore) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	if m.g == nil {
		m.g = map[string]float64{}
	}
	if m.c == nil {
		m.c = map[string]int64{}
	}
	return m.g, m.c, nil
}
func (m *memStore) UpdateMetricsBatch(ctx context.Context, metrics []*model.Metrics) error {
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
func (m *memStore) Ping(ctx context.Context) error { return nil }
func (m *memStore) Close() error                   { return nil }

var _ db.MetricsStorage = (*memStore)(nil)

func TestUpdateMetricJSON_Gauge(t *testing.T) {
	s := &memStore{}
	svc := NewMetricsService(s)
	v := 3.14
	m := &model.Metrics{ID: "g", MType: model.GaugeType, Value: &v}
	if err := svc.UpdateMetricJSON(m); err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.g["g"] != v {
		t.Fatalf("gauge not set")
	}
}

func TestUpdateMetricJSON_Counter(t *testing.T) {
	s := &memStore{}
	svc := NewMetricsService(s)
	d := int64(5)
	m := &model.Metrics{ID: "c", MType: model.CounterType, Delta: &d}
	if err := svc.UpdateMetricJSON(m); err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.c["c"] != d {
		t.Fatalf("counter not added")
	}
}
