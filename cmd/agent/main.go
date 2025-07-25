package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Heidric/metrics.git/internal/cfg"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/model"
	"github.com/rs/zerolog"
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
	databaseDSN := flag.String("d", config.DatabaseDSN, "Database DSN")

	flag.Parse()

	if *databaseDSN != "" {
		config.DatabaseDSN = *databaseDSN
	}

	return *serverAddr, time.Duration(*pollInterval) * time.Second, time.Duration(*reportInterval) * time.Second
}

func isRetriableHTTP(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func withRetryHTTP(client *http.Client, req *http.Request) (*http.Response, error) {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error
	for i, delay := range delays {
		resp, err := client.Do(req)
		if err == nil {
			return resp, nil
		}
		if !isRetriableHTTP(err) {
			return nil, err
		}
		lastErr = err
		if i < len(delays)-1 {
			time.Sleep(delay)
		}
	}
	return nil, fmt.Errorf("HTTP request failed after retries: %w", lastErr)
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
				{"Alloc", model.GaugeType, strconv.FormatFloat(float64(memStats.Alloc), 'f', -1, 64)},
				{"BuckHashSys", model.GaugeType, strconv.FormatFloat(float64(memStats.BuckHashSys), 'f', -1, 64)},
				{"Frees", model.GaugeType, strconv.FormatFloat(float64(memStats.Frees), 'f', -1, 64)},
				{"GCCPUFraction", model.GaugeType, strconv.FormatFloat(memStats.GCCPUFraction, 'f', -1, 64)},
				{"GCSys", model.GaugeType, strconv.FormatFloat(float64(memStats.GCSys), 'f', -1, 64)},
				{"HeapAlloc", model.GaugeType, strconv.FormatFloat(float64(memStats.HeapAlloc), 'f', -1, 64)},
				{"HeapIdle", model.GaugeType, strconv.FormatFloat(float64(memStats.HeapIdle), 'f', -1, 64)},
				{"HeapInuse", model.GaugeType, strconv.FormatFloat(float64(memStats.HeapInuse), 'f', -1, 64)},
				{"HeapObjects", model.GaugeType, strconv.FormatFloat(float64(memStats.HeapObjects), 'f', -1, 64)},
				{"HeapReleased", model.GaugeType, strconv.FormatFloat(float64(memStats.HeapReleased), 'f', -1, 64)},
				{"HeapSys", model.GaugeType, strconv.FormatFloat(float64(memStats.HeapSys), 'f', -1, 64)},
				{"LastGC", model.GaugeType, strconv.FormatFloat(float64(memStats.LastGC), 'f', -1, 64)},
				{"Lookups", model.GaugeType, strconv.FormatFloat(float64(memStats.Lookups), 'f', -1, 64)},
				{"MCacheInuse", model.GaugeType, strconv.FormatFloat(float64(memStats.MCacheInuse), 'f', -1, 64)},
				{"MCacheSys", model.GaugeType, strconv.FormatFloat(float64(memStats.MCacheSys), 'f', -1, 64)},
				{"MSpanInuse", model.GaugeType, strconv.FormatFloat(float64(memStats.MSpanInuse), 'f', -1, 64)},
				{"MSpanSys", model.GaugeType, strconv.FormatFloat(float64(memStats.MSpanSys), 'f', -1, 64)},
				{"Mallocs", model.GaugeType, strconv.FormatFloat(float64(memStats.Mallocs), 'f', -1, 64)},
				{"NextGC", model.GaugeType, strconv.FormatFloat(float64(memStats.NextGC), 'f', -1, 64)},
				{"NumForcedGC", model.GaugeType, strconv.FormatFloat(float64(memStats.NumForcedGC), 'f', -1, 64)},
				{"NumGC", model.GaugeType, strconv.FormatFloat(float64(memStats.NumGC), 'f', -1, 64)},
				{"OtherSys", model.GaugeType, strconv.FormatFloat(float64(memStats.OtherSys), 'f', -1, 64)},
				{"PauseTotalNs", model.GaugeType, strconv.FormatFloat(float64(memStats.PauseTotalNs), 'f', -1, 64)},
				{"StackInuse", model.GaugeType, strconv.FormatFloat(float64(memStats.StackInuse), 'f', -1, 64)},
				{"StackSys", model.GaugeType, strconv.FormatFloat(float64(memStats.StackSys), 'f', -1, 64)},
				{"Sys", model.GaugeType, strconv.FormatFloat(float64(memStats.Sys), 'f', -1, 64)},
				{"TotalAlloc", model.GaugeType, strconv.FormatFloat(float64(memStats.TotalAlloc), 'f', -1, 64)},
				{"RandomValue", model.GaugeType, strconv.FormatFloat(rand.Float64(), 'f', -1, 64)},
			}
			a.pollCountDelta++
			a.mu.Unlock()
		case <-a.stopChan:
			return
		}
	}
}

func (a *Agent) sendMetricsBatch(metrics []*model.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	compressed, err := a.compressData(data)
	if err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, a.serverURL+"/updates/", bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := withRetryHTTP(http.DefaultClient, req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return nil
}

func (a *Agent) prepareMetricsForBatch() []*model.Metrics {
	a.mu.Lock()
	defer a.mu.Unlock()

	var batch []*model.Metrics
	for _, m := range a.metrics {
		switch m.Type {
		case model.GaugeType:
			v, err := strconv.ParseFloat(m.Value, 64)
			if err != nil {
				continue
			}
			batch = append(batch, &model.Metrics{
				ID:    m.Name,
				MType: model.GaugeType,
				Value: &v,
			})
		case model.CounterType:
			d, err := strconv.ParseInt(m.Value, 10, 64)
			if err != nil {
				continue
			}
			batch = append(batch, &model.Metrics{
				ID:    m.Name,
				MType: model.CounterType,
				Delta: &d,
			})
		}
	}
	return batch
}

func (a *Agent) sendMetricsIndividually(metrics []*model.Metrics) {
	for _, m := range metrics {
		data, err := json.Marshal(m)
		if err != nil {
			logger.Log.Error().Msgf("Failed to marshal metric %s: %v", m.ID, err)
			continue
		}

		compressed, err := a.compressData(data)
		if err != nil {
			logger.Log.Error().Msgf("Failed to compress metric %s: %v", m.ID, err)
			continue
		}

		req, err := http.NewRequest(http.MethodPost, a.serverURL+"/update/", bytes.NewReader(compressed))
		if err != nil {
			logger.Log.Error().Msgf("Failed to create request for metric %s: %v", m.ID, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")

		resp, err := withRetryHTTP(http.DefaultClient, req)
		if err != nil {
			logger.Log.Error().Msgf("Failed to send metric %s: %v", m.ID, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Log.Error().Msgf("Server returned %s for metric %s", resp.Status, m.ID)
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
				Type:  model.CounterType,
				Value: fmt.Sprintf("%d", delta),
			}
			a.metrics = append(a.metrics, pollCountMetric)
			a.mu.Unlock()

			batch := a.prepareMetricsForBatch()
			err := a.sendMetricsBatch(batch)

			if err != nil {
				logger.Log.Error().Msgf("Batch failed: %v. Sending metrics one by one...", err)
				a.sendMetricsIndividually(batch)
			}

		case <-a.stopChan:
			return
		}
	}
}

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger.Log = &log

	serverAddr, pollInterval, reportInterval := parseFlags()

	agent := NewAgent(serverAddr, pollInterval, reportInterval)
	agent.Run()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	agent.Stop()
}
