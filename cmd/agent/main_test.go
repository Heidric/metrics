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
		name        string
		setup       func()
		wantAddress string
		wantPoll    time.Duration
		wantReport  time.Duration
	}{
		{
			name: "default values",
			setup: func() {
				os.Unsetenv("ADDRESS")
				os.Unsetenv("POLL_INTERVAL")
				os.Unsetenv("REPORT_INTERVAL")
				os.Unsetenv("DATABASE_DSN")
				os.Args = []string{"cmd"}
			},
			wantAddress: "localhost:8080",
			wantPoll:    2 * time.Second,
			wantReport:  10 * time.Second,
		},
		{
			name: "env variables",
			setup: func() {
				os.Setenv("ADDRESS", "env:8081")
				os.Setenv("POLL_INTERVAL", "3s")
				os.Setenv("REPORT_INTERVAL", "15s")
				os.Setenv("DATABASE_DSN", "env-dsn")
				os.Args = []string{"cmd"}
			},
			wantAddress: "env:8081",
			wantPoll:    3 * time.Second,
			wantReport:  15 * time.Second,
		},
		{
			name: "command line flags",
			setup: func() {
				os.Unsetenv("ADDRESS")
				os.Unsetenv("POLL_INTERVAL")
				os.Unsetenv("REPORT_INTERVAL")
				os.Unsetenv("DATABASE_DSN")
				os.Args = []string{"cmd", "-a=flag:8082", "-p=4", "-r=20"}
			},
			wantAddress: "flag:8082",
			wantPoll:    4 * time.Second,
			wantReport:  20 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			oldEnv := map[string]string{
				"ADDRESS":         os.Getenv("ADDRESS"),
				"POLL_INTERVAL":   os.Getenv("POLL_INTERVAL"),
				"REPORT_INTERVAL": os.Getenv("REPORT_INTERVAL"),
				"DATABASE_DSN":    os.Getenv("DATABASE_DSN"),
			}
			defer func() {
				os.Args = oldArgs
				for k, v := range oldEnv {
					os.Setenv(k, v)
				}
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			}()

			tt.setup()
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			address, poll, report := parseFlags()
			require.Equal(t, tt.wantAddress, address)
			require.Equal(t, tt.wantPoll, poll)
			require.Equal(t, tt.wantReport, report)
		})
	}
}
