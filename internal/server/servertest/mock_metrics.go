// Package servertest contains test helpers for internal/server.
package servertest

import (
	"context"

	"github.com/Heidric/metrics.git/internal/model"
)

// Metrics is the interface Server depends on.
type Metrics interface {
	ListMetrics() map[string]string
	GetMetric(metricType, metricName string) (string, error)
	UpdateGauge(name, value string) error
	UpdateCounter(name, value string) error
	UpdateMetricJSON(metric *model.Metrics) error
	GetMetricJSON(metric *model.Metrics) error
	UpdateMetricsBatch(metrics []*model.Metrics) error
	Ping(ctx context.Context) error
}

// MetricsMock is a function-backed mock for Metrics.
type MetricsMock struct {
	ListMetricsFn        func() map[string]string
	GetMetricFn          func(metricType, metricName string) (string, error)
	UpdateGaugeFn        func(name, value string) error
	UpdateCounterFn      func(name, value string) error
	UpdateMetricJSONFn   func(metric *model.Metrics) error
	GetMetricJSONFn      func(metric *model.Metrics) error
	UpdateMetricsBatchFn func(metrics []*model.Metrics) error
	PingFn               func(ctx context.Context) error
}

var _ Metrics = (*MetricsMock)(nil)

func NewMetricsMock(opts ...func(*MetricsMock)) *MetricsMock {
	m := &MetricsMock{
		ListMetricsFn:        func() map[string]string { return map[string]string{} },
		GetMetricFn:          func(string, string) (string, error) { return "", nil },
		UpdateGaugeFn:        func(string, string) error { return nil },
		UpdateCounterFn:      func(string, string) error { return nil },
		UpdateMetricJSONFn:   func(*model.Metrics) error { return nil },
		GetMetricJSONFn:      func(*model.Metrics) error { return nil },
		UpdateMetricsBatchFn: func([]*model.Metrics) error { return nil },
		PingFn:               func(context.Context) error { return nil },
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func WithListMetrics(fn func() map[string]string) func(*MetricsMock) {
	return func(m *MetricsMock) { m.ListMetricsFn = fn }
}
func WithGetMetric(fn func(string, string) (string, error)) func(*MetricsMock) {
	return func(m *MetricsMock) { m.GetMetricFn = fn }
}
func WithUpdateGauge(fn func(string, string) error) func(*MetricsMock) {
	return func(m *MetricsMock) { m.UpdateGaugeFn = fn }
}
func WithUpdateCounter(fn func(string, string) error) func(*MetricsMock) {
	return func(m *MetricsMock) { m.UpdateCounterFn = fn }
}
func WithUpdateMetricJSON(fn func(*model.Metrics) error) func(*MetricsMock) {
	return func(m *MetricsMock) { m.UpdateMetricJSONFn = fn }
}
func WithGetMetricJSON(fn func(*model.Metrics) error) func(*MetricsMock) {
	return func(m *MetricsMock) { m.GetMetricJSONFn = fn }
}
func WithUpdateMetricsBatch(fn func([]*model.Metrics) error) func(*MetricsMock) {
	return func(m *MetricsMock) { m.UpdateMetricsBatchFn = fn }
}
func WithPing(fn func(context.Context) error) func(*MetricsMock) {
	return func(m *MetricsMock) { m.PingFn = fn }
}

func (m *MetricsMock) ListMetrics() map[string]string        { return m.ListMetricsFn() }
func (m *MetricsMock) GetMetric(t, n string) (string, error) { return m.GetMetricFn(t, n) }
func (m *MetricsMock) UpdateGauge(n, v string) error         { return m.UpdateGaugeFn(n, v) }
func (m *MetricsMock) UpdateCounter(n, v string) error       { return m.UpdateCounterFn(n, v) }
func (m *MetricsMock) UpdateMetricJSON(metric *model.Metrics) error {
	return m.UpdateMetricJSONFn(metric)
}
func (m *MetricsMock) GetMetricJSON(metric *model.Metrics) error { return m.GetMetricJSONFn(metric) }
func (m *MetricsMock) UpdateMetricsBatch(metrics []*model.Metrics) error {
	return m.UpdateMetricsBatchFn(metrics)
}
func (m *MetricsMock) Ping(ctx context.Context) error { return m.PingFn(ctx) }
