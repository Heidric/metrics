package main

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		setup       func()
		wantAddress string
	}{
		{
			name: "default address",
			setup: func() {
				os.Unsetenv("ADDRESS")
				os.Args = []string{"cmd"}
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			},
			wantAddress: "localhost:8080",
		},
		{
			name: "env address",
			setup: func() {
				os.Setenv("ADDRESS", "env:8081")
				os.Args = []string{"cmd"}
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			},
			wantAddress: "env:8081",
		},
		{
			name: "flag address",
			setup: func() {
				os.Unsetenv("ADDRESS")
				os.Args = []string{"cmd", "-a=flag:8082"}
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			},
			wantAddress: "flag:8082",
		},
		{
			name: "flag overrides env",
			setup: func() {
				os.Setenv("ADDRESS", "env:8083")
				os.Args = []string{"cmd", "-a=flag:8084"}
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			},
			wantAddress: "flag:8084",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			oldEnv := os.Getenv("ADDRESS")
			oldFlags := flag.CommandLine

			defer func() {
				os.Args = oldArgs
				os.Setenv("ADDRESS", oldEnv)
				flag.CommandLine = oldFlags
			}()

			tt.setup()

			config, err := loadConfig()
			require.NoError(t, err)
			require.Equal(t, tt.wantAddress, config.ServerAddress)
		})
	}
}
