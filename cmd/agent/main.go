package main

import (
	"bytes"
	"compress/gzip"
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

func (a *Agent) compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	if _, err := gz.Write(data); err != nil {
		return nil, err
	}

	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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
				{"Alloc", "gauge", strconv.FormatFloat(float64(memStats.Alloc), 'f', -1, 64)},
				{"BuckHashSys", "gauge", strconv.FormatFloat(float64(memStats.BuckHashSys), 'f', -1, 64)},
				{"Frees", "gauge", strconv.FormatFloat(float64(memStats.Frees), 'f', -1, 64)},
				{"GCCPUFraction", "gauge", strconv.FormatFloat(memStats.GCCPUFraction, 'f', -1, 64)},
				{"GCSys", "gauge", strconv.FormatFloat(float64(memStats.GCSys), 'f', -1, 64)},
				{"HeapAlloc", "gauge", strconv.FormatFloat(float64(memStats.HeapAlloc), 'f', -1, 64)},
				{"HeapIdle", "gauge", strconv.FormatFloat(float64(memStats.HeapIdle), 'f', -1, 64)},
				{"HeapInuse", "gauge", strconv.FormatFloat(float64(memStats.HeapInuse), 'f', -1, 64)},
				{"HeapObjects", "gauge", strconv.FormatFloat(float64(memStats.HeapObjects), 'f', -1, 64)},
				{"HeapReleased", "gauge", strconv.FormatFloat(float64(memStats.HeapReleased), 'f', -1, 64)},
				{"HeapSys", "gauge", strconv.FormatFloat(float64(memStats.HeapSys), 'f', -1, 64)},
				{"LastGC", "gauge", strconv.FormatFloat(float64(memStats.LastGC), 'f', -1, 64)},
				{"Lookups", "gauge", strconv.FormatFloat(float64(memStats.Lookups), 'f', -1, 64)},
				{"MCacheInuse", "gauge", strconv.FormatFloat(float64(memStats.MCacheInuse), 'f', -1, 64)},
				{"MCacheSys", "gauge", strconv.FormatFloat(float64(memStats.MCacheSys), 'f', -1, 64)},
				{"MSpanInuse", "gauge", strconv.FormatFloat(float64(memStats.MSpanInuse), 'f', -1, 64)},
				{"MSpanSys", "gauge", strconv.FormatFloat(float64(memStats.MSpanSys), 'f', -1, 64)},
				{"Mallocs", "gauge", strconv.FormatFloat(float64(memStats.Mallocs), 'f', -1, 64)},
				{"NextGC", "gauge", strconv.FormatFloat(float64(memStats.NextGC), 'f', -1, 64)},
				{"NumForcedGC", "gauge", strconv.FormatFloat(float64(memStats.NumForcedGC), 'f', -1, 64)},
				{"NumGC", "gauge", strconv.FormatFloat(float64(memStats.NumGC), 'f', -1, 64)},
				{"OtherSys", "gauge", strconv.FormatFloat(float64(memStats.OtherSys), 'f', -1, 64)},
				{"PauseTotalNs", "gauge", strconv.FormatFloat(float64(memStats.PauseTotalNs), 'f', -1, 64)},
				{"StackInuse", "gauge", strconv.FormatFloat(float64(memStats.StackInuse), 'f', -1, 64)},
				{"StackSys", "gauge", strconv.FormatFloat(float64(memStats.StackSys), 'f', -1, 64)},
				{"Sys", "gauge", strconv.FormatFloat(float64(memStats.Sys), 'f', -1, 64)},
				{"TotalAlloc", "gauge", strconv.FormatFloat(float64(memStats.TotalAlloc), 'f', -1, 64)},
				{"RandomValue", "gauge", strconv.FormatFloat(rand.Float64(), 'f', -1, 64)},
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

				// Сжимаем данные перед отправкой
				compressedBody, err := a.compressData(body)
				if err != nil {
					continue
				}

				url := a.serverURL + "/update/"
				req, err := http.NewRequest("POST", url, bytes.NewReader(compressedBody))
				if err != nil {
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Content-Encoding", "gzip")
				req.Header.Set("Accept-Encoding", "gzip")

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
