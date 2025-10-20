package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/model"
	"github.com/Heidric/metrics.git/internal/services"
	"github.com/rs/zerolog"
)

// slowStore wraps db.Store and intentionally injects latency into write paths.
// This ensures requests remain in-flight during server shutdown, allowing us
// to validate http.Server's graceful shutdown behavior.
type slowStore struct {
	inner     *db.Store
	startOnce sync.Once
	started   chan struct{}
	delay     time.Duration
}

func newSlowStore(inner *db.Store, delay time.Duration) *slowStore {
	return &slowStore{
		inner:   inner,
		delay:   delay,
		started: make(chan struct{}),
	}
}

func (s *slowStore) notifyStart() {
	s.startOnce.Do(func() { close(s.started) })
}

// SetGauge applies an artificial delay and then delegates to the underlying store.
// It triggers the "started" signal to synchronize with the test's shutdown.
func (s *slowStore) SetGauge(ctx context.Context, name string, value float64) error {
	s.notifyStart()
	select {
	case <-ctx.Done():
	default:
	}
	time.Sleep(s.delay)
	return s.inner.SetGauge(ctx, name, value)
}

func (s *slowStore) GetGauge(ctx context.Context, name string) (float64, error) {
	return s.inner.GetGauge(ctx, name)
}

// SetCounter mirrors SetGauge with the same artificial delay.
func (s *slowStore) SetCounter(ctx context.Context, name string, value int64) error {
	s.notifyStart()
	time.Sleep(s.delay)
	return s.inner.SetCounter(ctx, name, value)
}

func (s *slowStore) GetCounter(ctx context.Context, name string) (int64, error) {
	return s.inner.GetCounter(ctx, name)
}

func (s *slowStore) GetAll(ctx context.Context) (map[string]float64, map[string]int64, error) {
	return s.inner.GetAll(ctx)
}

// UpdateMetricsBatch applies an artificial delay and delegates to the inner store.
func (s *slowStore) UpdateMetricsBatch(ctx context.Context, metrics []*model.Metrics) error {
	s.notifyStart()
	time.Sleep(s.delay)
	return s.inner.UpdateMetricsBatch(ctx, metrics)
}

func (s *slowStore) Ping(ctx context.Context) error {
	return s.inner.Ping(ctx)
}

// Close forwards to the inner store to force a final persistence when applicable.
func (s *slowStore) Close() error {
	return s.inner.Close()
}

// TestServer_GracefulShutdown_WaitsInFlightAndPersists verifies that:
//
//  1. The server accepts a batch update request and begins processing.
//  2. While the request is in-flight, Shutdown() is invoked.
//  3. http.Server Shutdown waits for the in-flight request to complete.
//  4. The underlying store contains the updated values after shutdown.
func TestServer_GracefulShutdown_WaitsInFlightAndPersists(t *testing.T) {
	l := zerolog.New(io.Discard).With().Timestamp().Logger()
	logger.Log = &l
	baseStore := db.NewStore("", 0)
	store := newSlowStore(baseStore, 350*time.Millisecond)

	metricsSvc := services.NewMetricsService(store)

	addr := "127.0.0.1:0"
	srv := NewServer(addr /*hashKey*/, "", metricsSvc)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	srv.Srv.Addr = ln.Addr().String()

	serverErr := make(chan error, 1)
	go func() {
		err := srv.Srv.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	client := &http.Client{Timeout: 5 * time.Second}

	{
		req, _ := http.NewRequest(http.MethodGet, "http://"+srv.Srv.Addr+"/ping", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("pre-ping request failed: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("pre-ping status = %d, want 200", resp.StatusCode)
		}
	}

	batch := []*model.Metrics{
		{ID: "g1", MType: model.GaugeType, Value: floatPtr(123.45)},
		{ID: "c1", MType: model.CounterType, Delta: int64Ptr(7)},
	}
	body, _ := json.Marshal(batch)

	postDone := make(chan *http.Response, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodPost, "http://"+srv.Srv.Addr+"/updates/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			postDone <- &http.Response{StatusCode: 0}
			return
		}
		postDone <- resp
	}()

	select {
	case <-store.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for store to start processing")
	}

	shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shCtx); err != nil {
		t.Fatalf("server shutdown failed: %v", err)
	}

	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for http.Server to exit after Shutdown")
	}

	select {
	case resp := <-postDone:
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("batch POST status = %d, want 200", resp.StatusCode)
		}
		_ = resp.Body.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for batch POST to complete")
	}

	{
		g, err := store.GetGauge(context.Background(), "g1")
		if err != nil {
			t.Fatalf("GetGauge: %v", err)
		}
		if g != 123.45 {
			t.Fatalf("gauge value = %v, want 123.45", g)
		}
		c, err := store.GetCounter(context.Background(), "c1")
		if err != nil {
			t.Fatalf("GetCounter: %v", err)
		}
		if c != 7 {
			t.Fatalf("counter value = %v, want 7", c)
		}
	}
}
