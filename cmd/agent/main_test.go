package main

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name          string
		setup         func()
		wantAddress   string
		wantPoll      time.Duration
		wantReport    time.Duration
		wantHashKey   string
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
				"ADDRESS":         os.Getenv("ADDRESS"),
				"POLL_INTERVAL":   os.Getenv("POLL_INTERVAL"),
				"REPORT_INTERVAL": os.Getenv("REPORT_INTERVAL"),
				"HASH_KEY":        os.Getenv("HASH_KEY"),
				"RATE_LIMIT":      os.Getenv("RATE_LIMIT"),
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
			address, poll, report, hashKey, rateLimit := parseFlags()
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
		name         string
		key          string
		defaultValue int
		envValue     string
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
			oldValue := os.Getenv(tt.key)
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
