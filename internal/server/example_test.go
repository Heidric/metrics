package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Heidric/metrics.git/internal/model"
)

type exampleMetrics struct{ store map[string]string }

func (m *exampleMetrics) ListMetrics() map[string]string { return m.store }
func (m *exampleMetrics) GetMetric(t, n string) (string, error) {
	v, ok := m.store[t+":"+n]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return v, nil
}
func (m *exampleMetrics) UpdateGauge(n, v string) error   { m.store["gauge:"+n] = v; return nil }
func (m *exampleMetrics) UpdateCounter(n, v string) error { m.store["counter:"+n] = v; return nil }
func (m *exampleMetrics) UpdateMetricJSON(metric *model.Metrics) error {
	if metric == nil {
		return fmt.Errorf("nil")
	}
	key := metric.MType + ":" + metric.ID
	if metric.MType == "gauge" && metric.Value != nil {
		m.store[key] = fmt.Sprintf("%f", *metric.Value)
		return nil
	}
	if metric.MType == "counter" && metric.Delta != nil {
		m.store[key] = fmt.Sprintf("%d", *metric.Delta)
		return nil
	}
	return fmt.Errorf("invalid")
}
func (m *exampleMetrics) GetMetricJSON(metric *model.Metrics) error {
	if metric == nil {
		return fmt.Errorf("nil")
	}
	key := metric.MType + ":" + metric.ID
	v, ok := m.store[key]
	if !ok {
		return fmt.Errorf("not found")
	}
	switch metric.MType {
	case "gauge":
		var f float64
		fmt.Sscanf(v, "%f", &f)
		metric.Value = &f
	case "counter":
		var d int64
		fmt.Sscanf(v, "%d", &d)
		metric.Delta = &d
	default:
		return fmt.Errorf("invalid")
	}
	return nil
}
func (m *exampleMetrics) UpdateMetricsBatch(metrics []*model.Metrics) error {
	for _, mt := range metrics {
		_ = m.UpdateMetricJSON(mt)
	}
	return nil
}
func (m *exampleMetrics) Ping(ctx context.Context) error { return nil }

// ExampleNewServer_path shows the path-based endpoints:
//
//	POST /update/{type}/{name}/{value} and GET /value/{type}/{name}.
func ExampleNewServer_path() {
	mem := &exampleMetrics{store: map[string]string{}}
	srv := NewServer(":0", "", mem)

	// POST /update/gauge/temp/21.5
	req := httptest.NewRequest(http.MethodPost, "/update/gauge/temp/21.5", nil)
	rec := httptest.NewRecorder()
	srv.Srv.Handler.ServeHTTP(rec, req)
	fmt.Println("POST status:", rec.Code)

	// GET /value/gauge/temp
	req2 := httptest.NewRequest(http.MethodGet, "/value/gauge/temp", nil)
	rec2 := httptest.NewRecorder()
	srv.Srv.Handler.ServeHTTP(rec2, req2)
	fmt.Println("GET status:", rec2.Code)
	body, _ := io.ReadAll(rec2.Body)
	fmt.Println("GET body:", strings.TrimSpace(string(body)))

	// Output:
	// POST status: 200
	// GET status: 200
	// GET body: 21.5
}

// ExampleNewServer_json shows the JSON endpoints: POST /update and POST /value.
func ExampleNewServer_json() {
	mem := &exampleMetrics{store: map[string]string{}}
	srv := NewServer(":0", "", mem)

	v := 3.14
	payload := model.Metrics{ID: "pi", MType: "gauge", Value: &v}
	b, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Srv.Handler.ServeHTTP(rec, req)
	fmt.Println("POST /update:", rec.Code)

	getPayload := model.Metrics{ID: "pi", MType: "gauge"}
	gb, _ := json.Marshal(getPayload)
	req2 := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(gb))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.Srv.Handler.ServeHTTP(rec2, req2)
	fmt.Println("POST /value:", rec2.Code)
	respBody, _ := io.ReadAll(rec2.Body)
	fmt.Println("Body has ID:", strings.Contains(string(respBody), "\"pi\""))

	// Output:
	// POST /update: 200
	// POST /value: 200
	// Body has ID: true
}

// ExampleNewServer_ping shows the health endpoint GET /ping.
func ExampleNewServer_ping() {
	mem := &exampleMetrics{store: map[string]string{}}
	srv := NewServer(":0", "", mem)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	srv.Srv.Handler.ServeHTTP(rec, req)
	fmt.Println(rec.Code)

	// Output:
	// 200
}
