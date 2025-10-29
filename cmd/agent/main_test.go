package main

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func runParseFlags(t *testing.T, args []string, env map[string]string) (addr string, poll, report time.Duration, hash string, rate int) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	savedFS := flag.CommandLine
	savedArgs := os.Args
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = append([]string{"agent"}, args...)
	defer func() { flag.CommandLine = savedFS; os.Args = savedArgs }()

	addr, poll, report, hash, rate, _, _, _, _ = parseFlags()
	return
}

func getenvOrEmpty(key string) string {
	v, _ := os.LookupEnv(key)
	return v
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		setup func()

		name        string
		wantAddress string
		wantHashKey string

		wantPoll   time.Duration
		wantReport time.Duration

		wantRateLimit int
	}{
		{
			name: "default values",
			setup: func() {
				os.Unsetenv("ADDRESS")
				os.Unsetenv("POLL_INTERVAL")
				os.Unsetenv("REPORT_INTERVAL")
				os.Unsetenv("HASH_KEY")
				os.Unsetenv("RATE_LIMIT")
				os.Args = []string{"cmd"}
			},
			wantAddress:   "localhost:8080",
			wantPoll:      2 * time.Second,
			wantReport:    10 * time.Second,
			wantHashKey:   "",
			wantRateLimit: 10,
		},
		{
			name: "env variables",
			setup: func() {
				os.Setenv("ADDRESS", "env:8081")
				// Надёжнее указывать единицы времени
				os.Setenv("POLL_INTERVAL", "3s")
				os.Setenv("REPORT_INTERVAL", "15s")
				os.Setenv("HASH_KEY", "hash-key")
				os.Setenv("RATE_LIMIT", "5")
				os.Args = []string{"cmd"}
			},
			wantAddress:   "env:8081",
			wantPoll:      3 * time.Second,
			wantReport:    15 * time.Second,
			wantHashKey:   "hash-key",
			wantRateLimit: 5,
		},
		{
			name: "command line flags",
			setup: func() {
				os.Unsetenv("ADDRESS")
				os.Unsetenv("POLL_INTERVAL")
				os.Unsetenv("REPORT_INTERVAL")
				os.Unsetenv("HASH_KEY")
				os.Unsetenv("RATE_LIMIT")
				os.Args = []string{"cmd", "-a=flag:8082", "-p=4", "-r=20", "-k=hash-key-cmd", "-l=15"}
			},
			wantAddress:   "flag:8082",
			wantPoll:      4 * time.Second,
			wantReport:    20 * time.Second,
			wantHashKey:   "hash-key-cmd",
			wantRateLimit: 15,
		},
		{
			name: "mixed env and flags (flags should override)",
			setup: func() {
				os.Setenv("ADDRESS", "env:8081")
				os.Setenv("POLL_INTERVAL", "3s")
				os.Setenv("REPORT_INTERVAL", "15s")
				os.Setenv("HASH_KEY", "hash-key-env")
				os.Setenv("RATE_LIMIT", "5")
				os.Args = []string{"cmd", "-a=flag:8082", "-l=20"}
			},
			wantAddress:   "flag:8082",
			wantPoll:      3 * time.Second,
			wantReport:    15 * time.Second,
			wantHashKey:   "hash-key-env",
			wantRateLimit: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			oldEnv := map[string]string{
				"ADDRESS":         getenvOrEmpty("ADDRESS"),
				"POLL_INTERVAL":   getenvOrEmpty("POLL_INTERVAL"),
				"REPORT_INTERVAL": getenvOrEmpty("REPORT_INTERVAL"),
				"HASH_KEY":        getenvOrEmpty("HASH_KEY"),
				"RATE_LIMIT":      getenvOrEmpty("RATE_LIMIT"),
			}
			defer func() {
				os.Args = oldArgs
				for k, v := range oldEnv {
					if v == "" {
						os.Unsetenv(k)
					} else {
						os.Setenv(k, v)
					}
				}
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			}()

			tt.setup()
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			address, poll, report, hashKey, rateLimit, _, _, _, _ := parseFlags()

			require.Equal(t, tt.wantAddress, address)
			require.Equal(t, tt.wantPoll, poll)
			require.Equal(t, tt.wantReport, report)
			require.Equal(t, tt.wantHashKey, hashKey)
			require.Equal(t, tt.wantRateLimit, rateLimit)
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envValue string

		defaultValue int
		want         int
	}{
		{
			name:         "env var not set",
			key:          "TEST_VAR",
			defaultValue: 42,
			envValue:     "",
			want:         42,
		},
		{
			name:         "env var set with valid int",
			key:          "TEST_VAR",
			defaultValue: 42,
			envValue:     "100",
			want:         100,
		},
		{
			name:         "env var set with invalid int",
			key:          "TEST_VAR",
			defaultValue: 42,
			envValue:     "not_a_number",
			want:         42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldValue := getenvOrEmpty(tt.key)
			defer func() {
				if oldValue == "" {
					os.Unsetenv(tt.key)
				} else {
					os.Setenv(tt.key, oldValue)
				}
			}()

			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			} else {
				os.Unsetenv(tt.key)
			}

			got := getEnvInt(tt.key, tt.defaultValue)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNewAgent(t *testing.T) {
	agent := NewAgent("localhost:8080", 2*time.Second, 10*time.Second, "test-key", 5)

	require.Equal(t, "http://localhost:8080", agent.serverURL)
	require.Equal(t, 2*time.Second, agent.pollInterval)
	require.Equal(t, 10*time.Second, agent.reportInterval)
	require.Equal(t, "test-key", agent.hashKey)
	require.Equal(t, 5, agent.rateLimit)
	require.NotNil(t, agent.client)
	require.NotNil(t, agent.jobChan)
	require.NotNil(t, agent.resultChan)
	require.NotNil(t, agent.stopChan)
}

func TestConvertToModelMetric(t *testing.T) {
	agent := NewAgent("localhost:8080", 2*time.Second, 10*time.Second, "test-key", 5)

	tests := []struct {
		name   string
		metric Metric
		want   bool
	}{
		{
			name: "valid gauge metric",
			metric: Metric{
				Name:  "TestGauge",
				Type:  "gauge",
				Value: "42.5",
			},
			want: false,
		},
		{
			name: "valid counter metric",
			metric: Metric{
				Name:  "TestCounter",
				Type:  "counter",
				Value: "100",
			},
			want: false,
		},
		{
			name: "invalid gauge value",
			metric: Metric{
				Name:  "TestGauge",
				Type:  "gauge",
				Value: "not_a_number",
			},
			want: true,
		},
		{
			name: "invalid counter value",
			metric: Metric{
				Name:  "TestCounter",
				Type:  "counter",
				Value: "not_a_number",
			},
			want: true,
		},
		{
			name: "unknown metric type",
			metric: Metric{
				Name:  "TestUnknown",
				Type:  "unknown",
				Value: "42",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.convertToModelMetric(tt.metric)
			if tt.want {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, tt.metric.Name, result.ID)
				require.Equal(t, tt.metric.Type, result.MType)
			}
		})
	}
}

func Test_getEnvInt(t *testing.T) {
	if got := getEnvInt("AGENT_TEST_INT_X", 42); got != 42 {
		t.Fatalf("unset -> default: %d, want 42", got)
	}
	t.Setenv("AGENT_TEST_INT_X", "17")
	if got := getEnvInt("AGENT_TEST_INT_X", 42); got != 17 {
		t.Fatalf("valid env: %d, want 17", got)
	}
	t.Setenv("AGENT_TEST_INT_X", "not-a-number")
	if got := getEnvInt("AGENT_TEST_INT_X", 42); got != 42 {
		t.Fatalf("invalid env -> default: %d, want 42", got)
	}
}

func Test_parseFlags_RateLimit_EnvOnly_NoFlag(t *testing.T) {
	_, _, _, _, rate := runParseFlags(t, []string{}, map[string]string{"RATE_LIMIT": "15"})
	if rate != 15 {
		t.Fatalf("rate=%d, want 15 (from env)", rate)
	}
}

func Test_parseFlags_RateLimit_FlagOverridesEnv(t *testing.T) {
	_, _, _, _, rate := runParseFlags(t, []string{"-l", "33"}, map[string]string{"RATE_LIMIT": "15"})
	if rate != 33 {
		t.Fatalf("rate=%d, want 33 (flag override)", rate)
	}
}

func Test_parseFlags_HashKey_EnvWhenNoFlag(t *testing.T) {
	_, _, _, hash, _ := runParseFlags(t, []string{}, map[string]string{"HASH_KEY": "env-secret"})
	if hash != "env-secret" {
		t.Fatalf("hash=%q, want %q (ENV)", hash, "env-secret")
	}
}

func Test_parseFlags_HashKey_FlagOverridesEnv(t *testing.T) {
	_, _, _, hash, _ := runParseFlags(t, []string{"-k", "flag-secret"}, map[string]string{"HASH_KEY": "env-secret"})
	if hash != "flag-secret" {
		t.Fatalf("hash=%q, want %q (flag override)", hash, "flag-secret")
	}
}

func Test_parseFlags_PollAndReport_FromEnvWhenNoFlags(t *testing.T) {
	addr, poll, report, _, _ := runParseFlags(t, []string{}, map[string]string{"POLL_INTERVAL": "3s", "REPORT_INTERVAL": "17s", "ADDRESS": "example:9090"})
	if poll != 3*time.Second || report != 17*time.Second {
		t.Fatalf("poll/report=%v/%v, want 3s/17s (ENV)", poll, report)
	}
	if addr != "example:9090" {
		t.Fatalf("addr=%q, want %q", addr, "example:9090")
	}
}

func Test_parseFlags_PollAndReport_FlagsOverrideEnv(t *testing.T) {
	_, poll, report, _, _ := runParseFlags(t, []string{"-p", "9", "-r", "21"}, map[string]string{"POLL_INTERVAL": "3s", "REPORT_INTERVAL": "17s"})
	if poll != 9*time.Second || report != 21*time.Second {
		t.Fatalf("poll/report=%v/%v, want 9s/21s (flags)", poll, report)
	}
}
