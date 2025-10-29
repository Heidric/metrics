package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Heidric/metrics.git/internal/buildinfo"
	"github.com/Heidric/metrics.git/internal/cfg"
	intcrypto "github.com/Heidric/metrics.git/internal/crypto"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/model"

	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	// gRPC
	"github.com/Heidric/metrics.git/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var (
	flagCryptoKey string // flag holder
	cryptoKeyPath string // effective path (flag > env)
	agentPubKey   *rsa.PublicKey
)

// Metric is a name/type/value object collected by the agent
// before it is converted into the model.Metrics.
type Metric struct {
	Name  string
	Type  string
	Value string
}

// MetricJob represents a single unit of work for the agent’s worker pool.
// Metric holds the payload to send; Ctx carries per-job cancellation/deadline.
type MetricJob struct {
	Metric *model.Metrics
	Ctx    context.Context
}

// Agent periodically collects runtime/system metrics and reports them to the server.
// Concurrency: collection and reporting run in goroutines; channels coordinate work.
type Agent struct {
	serverURL string
	hashKey   string

	// HTTP
	client *http.Client

	// gRPC
	useGRPC       bool
	grpcAddr      string
	grpcConn      *grpc.ClientConn
	grpcCli       pb.MetricsServiceClient
	grpcMaxRecvMB int
	grpcMaxSendMB int

	jobChan        chan MetricJob // inbound jobs for the worker pool
	resultChan     chan error     // reporting results (nil on success)
	stopChan       chan struct{}  // signals all goroutines to stop
	runtimeMetrics []Metric       // cached runtime metrics snapshot
	systemMetrics  []Metric       // cached system metrics snapshot

	pollInterval   time.Duration
	reportInterval time.Duration
	pollCountDelta int64 // aggregate delta for counter-type metrics

	rateLimit int

	mu     sync.RWMutex   // guards runtime/system metric caches and counters
	wg     sync.WaitGroup // waits for collectors/reporters to exit
	cancel context.CancelFunc
}

func encrypt(body []byte) (out []byte, encrypted bool, err error) {
	if agentPubKey == nil {
		return body, false, nil
	}
	env, err := intcrypto.EncryptFor(agentPubKey, body)
	if err != nil {
		return nil, false, err
	}
	b, err := json.Marshal(env)
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func parseFlags() (serverAddr string, pollInterval time.Duration, reportInterval time.Duration, hashKey string, rateLimit int, grpcEnabled bool, grpcAddr string, grpcMaxRecvMB int, grpcMaxSendMB int) {
	config, err := cfg.NewConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v\n", err)
	}

	flag.String("config", "", "path to JSON config file")
	flag.String("c", "", "path to JSON config file (shorthand)")
	serverAddrPtr := flag.String("a", config.ServerAddress, "HTTP server endpoint address (host:port or scheme://host:port)")
	pollIntervalPtr := flag.Int("p", int(config.PollInterval.Seconds()), "Poll interval in seconds")
	reportIntervalPtr := flag.Int("r", int(config.ReportInterval.Seconds()), "Report interval in seconds")
	databaseDSN := flag.String("d", config.DatabaseDSN, "Database DSN")
	hashKeyPtr := flag.String("k", config.HashKey, "Hash key")
	rateLimitPtr := flag.Int("l", getEnvInt("RATE_LIMIT", 10), "Rate limit for concurrent requests")
	flag.StringVar(&flagCryptoKey, "crypto-key", "", "path to RSA public key (PEM)")

	grpcEnabledPtr := flag.Bool("grpc", config.GRPCEnabled, "Enable gRPC client")
	grpcAddrPtr := flag.String("grpc-addr", config.GRPCAddress, "gRPC server address (host:port)")
	grpcMaxRecvPtr := flag.Int("grpc-max-recv", config.GRPCMaxRecvMB, "gRPC max recv size in MB")
	grpcMaxSendPtr := flag.Int("grpc-max-send", config.GRPCMaxSendMB, "gRPC max send size in MB")

	flag.Parse()

	// Prefer flag; otherwise accept explicit env override including empty string.
	if flagCryptoKey != "" {
		cryptoKeyPath = flagCryptoKey
	} else if v, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		cryptoKeyPath = v
	}

	if *databaseDSN != "" {
		config.DatabaseDSN = *databaseDSN
	}

	return *serverAddrPtr,
		time.Duration(*pollIntervalPtr) * time.Second,
		time.Duration(*reportIntervalPtr) * time.Second,
		*hashKeyPtr,
		*rateLimitPtr,
		*grpcEnabledPtr,
		*grpcAddrPtr,
		*grpcMaxRecvPtr,
		*grpcMaxSendPtr
}

func getEnvInt(key string, defaultValue int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultValue
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

// NewAgent constructs an Agent with the given server endpoint, polling/reporting
// intervals, optional HMAC key, and a concurrency rate limit.
// serverURL is host:port (scheme is prefixed as "http://").
func NewAgent(serverURL string, pollInterval, reportInterval time.Duration, hashKey string, rateLimit int) *Agent {
	return &Agent{
		serverURL:      "http://" + serverURL,
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		hashKey:        hashKey,
		rateLimit:      rateLimit,
		client:         &http.Client{Timeout: 5 * time.Second},
		jobChan:        make(chan MetricJob, 100),
		resultChan:     make(chan error, 100),
		stopChan:       make(chan struct{}),
	}
}

// initGRPC dials the gRPC server if enabled.
func (a *Agent) initGRPC() error {
	if !a.useGRPC || a.grpcAddr == "" {
		return nil
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(a.grpcMaxRecvMB*1024*1024),
			grpc.MaxCallSendMsgSize(a.grpcMaxSendMB*1024*1024),
		),
	}
	conn, err := grpc.NewClient(a.grpcAddr, opts...)
	if err != nil {
		return err
	}
	a.grpcConn = conn
	a.grpcCli = pb.NewMetricsServiceClient(conn)
	return nil
}

// Run starts the agent’s worker pool, collectors, and reporting loops.
// It returns immediately; goroutines keep running until Stop is called.
func (a *Agent) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	if a.useGRPC {
		if err := a.initGRPC(); err != nil {
			logger.Log.Error().Msgf("gRPC disabled: dial failed: %v", err)
			a.useGRPC = false
		}
	}

	a.startWorkerPool(ctx)

	a.wg.Add(3)
	go a.pollRuntimeMetrics()
	go a.pollSystemMetrics()
	go a.reportMetrics()

	go a.processResults()

	<-a.stopChan
}

// Stop signals all goroutines to exit and waits for them to finish.
// Channels are closed after all workers have drained.
func (a *Agent) Stop() {
	close(a.stopChan)
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	close(a.jobChan)
	close(a.resultChan)
	if a.grpcConn != nil {
		_ = a.grpcConn.Close()
	}
}

func (a *Agent) startWorkerPool(ctx context.Context) {
	for i := 0; i < a.rateLimit; i++ {
		go a.worker(ctx)
	}
}

func (a *Agent) worker(ctx context.Context) {
	for {
		select {
		case job, ok := <-a.jobChan:
			if !ok {
				return
			}
			err := a.sendMetric(job.Ctx, job.Metric)
			select {
			case a.resultChan <- err:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (a *Agent) processResults() {
	for err := range a.resultChan {
		if err != nil {
			logger.Log.Error().Msgf("Metric sending failed: %v", err)
		}
	}
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

// localIPForServer detects the local IP that would be used to reach the HTTP server.
func (a *Agent) localIPForServer() string {
	u, err := url.Parse(a.serverURL)
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "80")
	}
	return localIPForHostPort(host)
}

// localIPForHostPort detects local IP for reaching a host:port over UDP.
func localIPForHostPort(hostport string) string {
	conn, err := net.Dial("udp", hostport)
	if err != nil {
		return ""
	}
	defer conn.Close()
	if la, ok := conn.LocalAddr().(*net.UDPAddr); ok && la.IP != nil {
		if ip4 := la.IP.To4(); ip4 != nil {
			return ip4.String()
		}
		return la.IP.String()
	}
	return ""
}

// localIPBest tries HTTP server target first, then gRPC address.
func (a *Agent) localIPBest() string {
	if ip := a.localIPForServer(); ip != "" {
		return ip
	}
	if a.grpcAddr != "" {
		return localIPForHostPort(a.grpcAddr)
	}
	return ""
}

func (a *Agent) sendMetric(ctx context.Context, metric *model.Metrics) error {
	if a.useGRPC {
		return a.sendMetricGRPC(ctx, metric)
	}

	data, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}

	payload := data
	encrypted := false
	if agentPubKey != nil {
		encBody, enc, encErr := encrypt(data)
		if encErr != nil {
			return fmt.Errorf("encryption failed: %w", encErr)
		}
		payload = encBody
		encrypted = enc
	}

	if a.hashKey != "" {
		_ = intcrypto.HashSHA256(payload, a.hashKey)
	}

	compressed, err := a.compressData(payload)
	if err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.serverURL+"/update/", bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	if encrypted {
		req.Header.Set("X-Encrypted", "1")
	}
	if ip := a.localIPBest(); ip != "" {
		req.Header.Set("X-Real-IP", ip)
	}

	if a.hashKey != "" {
		req.Header.Set("HashSHA256", intcrypto.HashSHA256(payload, a.hashKey))
	}

	resp, err := withRetryHTTP(a.client, req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return nil
}

// sendMetricGRPC sends a metric via gRPC using the pb API.
func (a *Agent) sendMetricGRPC(ctx context.Context, metric *model.Metrics) error {
	if a.grpcCli == nil {
		return errors.New("grpc not initialized")
	}
	pm := &pb.Metric{Id: metric.ID, Type: metric.MType}
	if metric.MType == model.GaugeType && metric.Value != nil {
		pm.Value = *metric.Value
	}
	if metric.MType == model.CounterType && metric.Delta != nil {
		pm.Delta = *metric.Delta
	}
	ip := a.localIPBest()
	md := metadata.MD{}
	if ip != "" {
		md.Set("x-real-ip", ip)
	}
	ctx = metadata.NewOutgoingContext(ctx, md)
	_, err := a.grpcCli.Update(ctx, &pb.UpdateRequest{Metric: pm})
	return err
}

func (a *Agent) pollRuntimeMetrics() {
	defer a.wg.Done()
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			a.mu.Lock()
			a.runtimeMetrics = []Metric{
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

func (a *Agent) pollSystemMetrics() {
	defer a.wg.Done()
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var systemMetrics []Metric

			if memInfo, err := mem.VirtualMemory(); err == nil {
				systemMetrics = append(systemMetrics,
					Metric{"TotalMemory", model.GaugeType, strconv.FormatFloat(float64(memInfo.Total), 'f', -1, 64)},
					Metric{"FreeMemory", model.GaugeType, strconv.FormatFloat(float64(memInfo.Free), 'f', -1, 64)},
				)
			}

			if cpuPercents, err := cpu.Percent(0, true); err == nil {
				for i, percent := range cpuPercents {
					metricName := fmt.Sprintf("CPUutilization%d", i+1)
					systemMetrics = append(systemMetrics,
						Metric{metricName, model.GaugeType, strconv.FormatFloat(percent, 'f', -1, 64)},
					)
				}
			}

			a.mu.Lock()
			a.systemMetrics = systemMetrics
			a.mu.Unlock()
		case <-a.stopChan:
			return
		}
	}
}

func (a *Agent) reportMetrics() {
	defer a.wg.Done()
	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.mu.Lock()
			delta := a.pollCountDelta
			a.pollCountDelta = 0

			allMetrics := make([]Metric, 0, len(a.runtimeMetrics)+len(a.systemMetrics)+1)
			allMetrics = append(allMetrics, a.runtimeMetrics...)
			allMetrics = append(allMetrics, a.systemMetrics...)
			allMetrics = append(allMetrics, Metric{
				Name:  "PollCount",
				Type:  model.CounterType,
				Value: fmt.Sprintf("%d", delta),
			})
			a.mu.Unlock()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			for _, m := range allMetrics {
				modelMetric := a.convertToModelMetric(m)
				if modelMetric != nil {
					select {
					case a.jobChan <- MetricJob{Metric: modelMetric, Ctx: ctx}:
					case <-ctx.Done():
						logger.Log.Error().Msg("Context timeout while sending metrics")
						break
					}
				}
			}
			cancel()
		case <-a.stopChan:
			return
		}
	}
}

func (a *Agent) convertToModelMetric(m Metric) *model.Metrics {
	switch m.Type {
	case model.GaugeType:
		v, err := strconv.ParseFloat(m.Value, 64)
		if err != nil {
			return nil
		}
		return &model.Metrics{
			ID:    m.Name,
			MType: model.GaugeType,
			Value: &v,
		}
	case model.CounterType:
		d, err := strconv.ParseInt(m.Value, 10, 64)
		if err != nil {
			return nil
		}
		return &model.Metrics{
			ID:    m.Name,
			MType: model.CounterType,
			Delta: &d,
		}
	}
	return nil
}

func main() {
	buildinfo.PrintStdout()

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger.Log = &log

	serverAddr, pollInterval, reportInterval, hashKey, rateLimit, grpcEnabled, grpcAddr, grpcMaxRecv, grpcMaxSend :=
		parseFlags()

	if cryptoKeyPath != "" {
		k, err := intcrypto.ParseRSAPublicKeyPEM(cryptoKeyPath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load RSA public key")
		}
		agentPubKey = k
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	agent := NewAgent(serverAddr, pollInterval, reportInterval, hashKey, rateLimit)
	agent.useGRPC = grpcEnabled
	agent.grpcAddr = grpcAddr
	if agent.grpcAddr == "" {
		agent.grpcAddr = "localhost:9090"
	}
	if grpcMaxRecv <= 0 {
		grpcMaxRecv = 8
	}
	if grpcMaxSend <= 0 {
		grpcMaxSend = 8
	}
	agent.grpcMaxRecvMB = grpcMaxRecv
	agent.grpcMaxSendMB = grpcMaxSend

	go agent.Run()

	<-ctx.Done()
	agent.Stop()
}
