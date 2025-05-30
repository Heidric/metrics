package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Heidric/metrics.git/internal/cfg"
	"github.com/Heidric/metrics.git/internal/model"
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
	pollCountDelta int64
	client         *http.Client
	mu             sync.Mutex
	stopChan       chan struct{}
}

func parseFlags() (string, time.Duration, time.Duration) {
	config, err := cfg.NewConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	serverAddr := flag.String("a", config.ServerAddress, "HTTP server endpoint address")
	pollInterval := flag.Int("p", int(config.PollInterval.Seconds()), "Poll interval in seconds")
	reportInterval := flag.Int("r", int(config.ReportInterval.Seconds()), "Report interval in seconds")

	flag.Parse()

	return *serverAddr, time.Duration(*pollInterval) * time.Second, time.Duration(*reportInterval) * time.Second
}

func NewAgent(serverURL string, pollInterval, reportInterval time.Duration) *Agent {
	return &Agent{
		serverURL:      "http://" + serverURL,
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
			a.pollCountDelta++
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
			delta := a.pollCountDelta
			a.pollCountDelta = 0
			pollCountMetric := Metric{
				Name:  "PollCount",
				Type:  "counter",
				Value: fmt.Sprintf("%d", delta),
			}
			allMetrics := append(a.metrics, pollCountMetric)
			a.mu.Unlock()

			for _, metric := range allMetrics {
				var m model.Metrics
				var body []byte
				var err error

				if metric.Type == "counter" {
					delta, _ := strconv.ParseInt(metric.Value, 10, 64)
					m = model.Metrics{
						ID:    metric.Name,
						MType: metric.Type,
						Delta: &delta,
					}
					body, err = json.Marshal(m)
				} else {
					value, _ := strconv.ParseFloat(metric.Value, 64)
					m = model.Metrics{
						ID:    metric.Name,
						MType: metric.Type,
						Value: &value,
					}
					body, err = json.Marshal(m)
				}

				if err != nil {
					continue
				}

				url := a.serverURL + "/update/"
				req, err := http.NewRequest("POST", url, bytes.NewReader(body))
				if err != nil {
					continue
				}
				req.Header.Set("Content-Type", "application/json")

				resp, err := a.client.Do(req)
				if err != nil {
					continue
				}
				resp.Body.Close()
			}
		case <-a.stopChan:
			return
		}
	}
}

func main() {
	serverAddr, pollInterval, reportInterval := parseFlags()

	agent := NewAgent(serverAddr, pollInterval, reportInterval)
	agent.Run()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	agent.Stop()
}
