package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

type Metric struct {
	Name  string
	Type  string
	Value string
}

type Agent struct {
	serverURL      string
	pollInterval   time.Duration
	reportInterval time.Duration
	metrics        []Metric
	pollCount      int64
	client         *http.Client
	mu             sync.Mutex
	stopChan       chan struct{}
}

func NewAgent(serverURL string, pollInterval, reportInterval time.Duration) *Agent {
	return &Agent{
		serverURL:      serverURL,
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		client:         &http.Client{Timeout: 5 * time.Second},
		stopChan:       make(chan struct{}),
	}
}

func (a *Agent) Run() {
	go a.pollMetrics()
	go a.reportMetrics()
}

func (a *Agent) Stop() {
	close(a.stopChan)
}

func (a *Agent) pollMetrics() {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			a.mu.Lock()
			a.metrics = []Metric{
				{"Alloc", "gauge", fmt.Sprintf("%f", float64(memStats.Alloc))},
				{"BuckHashSys", "gauge", fmt.Sprintf("%f", float64(memStats.BuckHashSys))},
				{"Frees", "gauge", fmt.Sprintf("%f", float64(memStats.Frees))},
				{"GCCPUFraction", "gauge", fmt.Sprintf("%f", memStats.GCCPUFraction)},
				{"GCSys", "gauge", fmt.Sprintf("%f", float64(memStats.GCSys))},
				{"HeapAlloc", "gauge", fmt.Sprintf("%f", float64(memStats.HeapAlloc))},
				{"HeapIdle", "gauge", fmt.Sprintf("%f", float64(memStats.HeapIdle))},
				{"HeapInuse", "gauge", fmt.Sprintf("%f", float64(memStats.HeapInuse))},
				{"HeapObjects", "gauge", fmt.Sprintf("%f", float64(memStats.HeapObjects))},
				{"HeapReleased", "gauge", fmt.Sprintf("%f", float64(memStats.HeapReleased))},
				{"HeapSys", "gauge", fmt.Sprintf("%f", float64(memStats.HeapSys))},
				{"LastGC", "gauge", fmt.Sprintf("%f", float64(memStats.LastGC))},
				{"Lookups", "gauge", fmt.Sprintf("%f", float64(memStats.Lookups))},
				{"MCacheInuse", "gauge", fmt.Sprintf("%f", float64(memStats.MCacheInuse))},
				{"MCacheSys", "gauge", fmt.Sprintf("%f", float64(memStats.MCacheSys))},
				{"MSpanInuse", "gauge", fmt.Sprintf("%f", float64(memStats.MSpanInuse))},
				{"MSpanSys", "gauge", fmt.Sprintf("%f", float64(memStats.MSpanSys))},
				{"Mallocs", "gauge", fmt.Sprintf("%f", float64(memStats.Mallocs))},
				{"NextGC", "gauge", fmt.Sprintf("%f", float64(memStats.NextGC))},
				{"NumForcedGC", "gauge", fmt.Sprintf("%f", float64(memStats.NumForcedGC))},
				{"NumGC", "gauge", fmt.Sprintf("%f", float64(memStats.NumGC))},
				{"OtherSys", "gauge", fmt.Sprintf("%f", float64(memStats.OtherSys))},
				{"PauseTotalNs", "gauge", fmt.Sprintf("%f", float64(memStats.PauseTotalNs))},
				{"StackInuse", "gauge", fmt.Sprintf("%f", float64(memStats.StackInuse))},
				{"StackSys", "gauge", fmt.Sprintf("%f", float64(memStats.StackSys))},
				{"Sys", "gauge", fmt.Sprintf("%f", float64(memStats.Sys))},
				{"TotalAlloc", "gauge", fmt.Sprintf("%f", float64(memStats.TotalAlloc))},
				{"RandomValue", "gauge", fmt.Sprintf("%f", rand.Float64())},
			}
			a.pollCount++
			a.mu.Unlock()
		case <-a.stopChan:
			return
		}
	}
}

func (a *Agent) reportMetrics() {
	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.mu.Lock()
			pollCountMetric := Metric{
				Name:  "PollCount",
				Type:  "counter",
				Value: fmt.Sprintf("%d", a.pollCount),
			}
			allMetrics := append(a.metrics, pollCountMetric)
			a.mu.Unlock()

			for _, metric := range allMetrics {
				url := fmt.Sprintf("%s/update/%s/%s/%s", a.serverURL, metric.Type, metric.Name, metric.Value)
				req, err := http.NewRequest("POST", url, bytes.NewBufferString(metric.Value))
				if err != nil {
					continue
				}
				req.Header.Set("Content-Type", "text/plain")

				resp, err := a.client.Do(req)
				if err != nil {
					continue
				}
				err = resp.Body.Close()
				if err != nil {
					continue
				}
			}
		case <-a.stopChan:
			return
		}
	}
}

func (a *Agent) GetMetrics() []Metric {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.metrics
}

func (a *Agent) GetPollCount() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pollCount
}

func main() {
	agent := NewAgent("http://localhost:8080", 2*time.Second, 10*time.Second)
	agent.Run()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	agent.Stop()
}
