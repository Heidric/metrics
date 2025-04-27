package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAgent_CollectMetrics(t *testing.T) {
	agent := NewAgent("http://localhost:8080", 10*time.Millisecond, 100*time.Millisecond)
	defer agent.Stop()

	go agent.pollMetrics()

	time.Sleep(50 * time.Millisecond)

	metrics := agent.GetMetrics()
	if len(metrics) == 0 {
		t.Error("Expected metrics to be collected")
	}

	foundRandom := false
	for _, m := range metrics {
		if m.Name == "RandomValue" {
			foundRandom = true
			break
		}
	}
	if !foundRandom {
		t.Error("RandomValue metric not found")
	}

	if agent.GetPollCount() == 0 {
		t.Error("Expected pollCount to be greater than 0")
	}
}

func TestAgent_SendMetrics(t *testing.T) {
	requestReceived := make(chan struct{})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestReceived)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	agent := NewAgent(ts.URL, 10*time.Millisecond, 10*time.Millisecond)
	defer agent.Stop()

	agent.mu.Lock()
	agent.metrics = []Metric{{"TestMetric", "gauge", "123.45"}}
	agent.pollCount = 42
	agent.mu.Unlock()

	go agent.reportMetrics()

	select {
	case <-requestReceived:
	case <-time.After(500 * time.Millisecond):
		t.Error("Timed out waiting for request")
	}
}

func TestNewAgent(t *testing.T) {
	agent := NewAgent("http://test", 1*time.Second, 2*time.Second)
	defer agent.Stop()

	if agent.serverURL != "http://test" {
		t.Errorf("Expected serverURL 'http://test', got '%s'", agent.serverURL)
	}

	if agent.pollInterval != 1*time.Second {
		t.Errorf("Expected pollInterval 1s, got %v", agent.pollInterval)
	}

	if agent.reportInterval != 2*time.Second {
		t.Errorf("Expected reportInterval 2s, got %v", agent.reportInterval)
	}

	if agent.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}
}
