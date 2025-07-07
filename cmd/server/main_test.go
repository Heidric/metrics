package main

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name            string
		setup           func()
		wantAddress     string
		wantDatabaseDSN string
	}{
		{
			name: "default address",
			setup: func() {
				os.Clearenv()
				os.Args = []string{"cmd"}
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			},
			wantAddress:     "localhost:8080",
			wantDatabaseDSN: "",
		},
		{
			name: "flag address",
			setup: func() {
				os.Clearenv()
				os.Args = []string{"cmd", "-a=flag:8082", "-d=flag-dsn"}
				flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			},
			wantAddress:     "flag:8082",
			wantDatabaseDSN: "flag-dsn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			oldFlags := flag.CommandLine

			defer func() {
				os.Args = oldArgs
				flag.CommandLine = oldFlags
			}()

			tt.setup()

			config, err := loadConfig()
			require.NoError(t, err)
			require.Equal(t, tt.wantAddress, config.ServerAddress)
			require.Equal(t, tt.wantDatabaseDSN, config.DatabaseDSN)
		})
	}
}
