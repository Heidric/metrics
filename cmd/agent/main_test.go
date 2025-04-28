package main

import (
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestGetMetrics(t *testing.T) {
	agent := NewAgent("localhost:8080", time.Second, time.Second)
	defer agent.Stop()

	agent.mu.Lock()
	agent.metrics = []Metric{
		{"TestMetric1", "gauge", "1.23"},
		{"TestMetric2", "counter", "42"},
	}
	agent.pollCount = 10
	agent.mu.Unlock()

	metrics := agent.GetMetrics()
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(metrics))
	}

	if count := agent.GetPollCount(); count != 10 {
		t.Errorf("Expected poll count 10, got %d", count)
	}
}

func TestParseFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name       string
		args       []string
		wantAddr   string
		wantPoll   time.Duration
		wantReport time.Duration
	}{
		{
			name:       "default values",
			args:       []string{"cmd"},
			wantAddr:   "localhost:8080",
			wantPoll:   2 * time.Second,
			wantReport: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			addr, poll, report := parseFlags()
			if addr != tt.wantAddr {
				t.Errorf("parseFlags() addr = %v, want %v", addr, tt.wantAddr)
			}
			if poll != tt.wantPoll {
				t.Errorf("parseFlags() poll = %v, want %v", poll, tt.wantPoll)
			}
			if report != tt.wantReport {
				t.Errorf("parseFlags() report = %v, want %v", report, tt.wantReport)
			}
		})
	}
}

func TestAgent_CollectMetrics(t *testing.T) {
	agent := NewAgent("localhost:8080", 10*time.Millisecond, 100*time.Millisecond)
	defer agent.Stop()

	go agent.pollMetrics()

	time.Sleep(50 * time.Millisecond)

	metrics := agent.GetMetrics()
	if len(metrics) == 0 {
		t.Error("Expected metrics to be collected")
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

	agent := NewAgent(ts.URL[len("http://"):], 10*time.Millisecond, 10*time.Millisecond)
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
	agent := NewAgent("localhost:8080", time.Second, 2*time.Second)
	defer agent.Stop()

	if agent.serverURL != "http://localhost:8080" {
		t.Errorf("Expected serverURL 'http://localhost:8080', got '%s'", agent.serverURL)
	}
	if agent.pollInterval != time.Second {
		t.Errorf("Expected pollInterval 1s, got %v", agent.pollInterval)
	}
	if agent.reportInterval != 2*time.Second {
		t.Errorf("Expected reportInterval 2s, got %v", agent.reportInterval)
	}
	if agent.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}
}
