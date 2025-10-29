package main

import (
	"flag"
	"os"
	"testing"
	"time"

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

func runLoadConfigWithArgs(t *testing.T, args []string, env map[string]string) *Config {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	savedFS := flag.CommandLine
	savedArgs := os.Args
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = append([]string{"cmd"}, args...)
	defer func() { flag.CommandLine = savedFS; os.Args = savedArgs }()

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	return cfg
}

func TestLoadConfig_FlagsOverrideEnv_Address(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{"-a", "0.0.0.0:9000"},
		map[string]string{"ADDRESS": "127.0.0.1:1234"},
	)
	if cfg.ServerAddress != "0.0.0.0:9000" {
		t.Fatalf("ServerAddress=%q, want %q", cfg.ServerAddress, "0.0.0.0:9000")
	}
}

func TestLoadConfig_Restore_EnvOnly_NoFlag(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{},
		map[string]string{"RESTORE": "false"},
	)
	if cfg.Restore != false {
		t.Fatalf("Restore=%v, want false (from env)", cfg.Restore)
	}
}

func TestLoadConfig_Restore_FlagOverridesEnv(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{"-r", "true"},
		map[string]string{"RESTORE": "false"},
	)
	if cfg.Restore != true {
		t.Fatalf("Restore=%v, want true (flag override)", cfg.Restore)
	}
}

func TestLoadConfig_StoreInterval_FlagOverridesEnv(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{"-i", "7s"},
		map[string]string{"STORE_INTERVAL": "600"},
	)
	if cfg.StoreInterval != 7*time.Second {
		t.Fatalf("StoreInterval=%v, want 7s", cfg.StoreInterval)
	}
}

func TestLoadConfig_Address_EnvWhenNoFlag(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{},
		map[string]string{"ADDRESS": "127.0.0.1:4242"},
	)
	if cfg.ServerAddress != "127.0.0.1:4242" {
		t.Fatalf("ServerAddress=%q, want %q (ENV)", cfg.ServerAddress, "127.0.0.1:4242")
	}
}

func TestLoadConfig_DSN_FlagOverridesEnv(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{"-d", "postgres://flag-user:pw@h:5432/flagdb?sslmode=disable"},
		map[string]string{"DATABASE_DSN": "postgres://env-user:pw@h:5432/envdb?sslmode=disable"},
	)
	want := "postgres://flag-user:pw@h:5432/flagdb?sslmode=disable"
	if cfg.DatabaseDSN != want {
		t.Fatalf("DatabaseDSN=%q, want %q (flag override)", cfg.DatabaseDSN, want)
	}
}

func TestLoadConfig_FileStoragePath_EnvWhenNoFlag(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{},
		map[string]string{"STORE_FILE": "/var/lib/metrics.json"},
	)
	if cfg.FileStoragePath != "/var/lib/metrics.json" {
		t.Fatalf("FileStoragePath=%q, want %q (ENV)", cfg.FileStoragePath, "/var/lib/metrics.json")
	}
}

func TestLoadConfig_HashKey_FlagOverridesEnv(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{"-k", "flag-secret"},
		map[string]string{"HASH_KEY": "env-secret"},
	)
	if cfg.HashKey != "flag-secret" {
		t.Fatalf("HashKey=%q, want %q (flag override)", cfg.HashKey, "flag-secret")
	}
}

func TestLoadConfig_StoreInterval_EnvWhenNoFlag(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{},
		map[string]string{"STORE_INTERVAL": "42"},
	)
	if cfg.StoreInterval != 42*time.Second {
		t.Fatalf("StoreInterval=%v, want 42s (ENV)", cfg.StoreInterval)
	}
}

func TestLoadConfig_GRPC_Defaults(t *testing.T) {
	os.Clearenv()
	cfg := runLoadConfigWithArgs(t, []string{}, nil)

	if cfg.GRPCEnabled != false {
		t.Fatalf("GRPCEnabled=%v, want false (default)", cfg.GRPCEnabled)
	}
	if cfg.GRPCAddress == "" {
		t.Fatalf("GRPCAddress empty, want default like :9090")
	}
	if cfg.GRPCMaxRecvMB <= 0 || cfg.GRPCMaxSendMB <= 0 {
		t.Fatalf("GRPCMaxRecv/Send must be >0, got %d/%d", cfg.GRPCMaxRecvMB, cfg.GRPCMaxSendMB)
	}
	// reflection default comes from cfg: usually true
	if !cfg.GRPCReflection {
		t.Fatalf("GRPCReflection=%v, want true (default)", cfg.GRPCReflection)
	}
}

func TestLoadConfig_GRPC_EnableByFlagAndSizes(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{"-grpc", "-grpc-addr", ":5051", "-grpc-max-recv", "16", "-grpc-max-send", "32", "-grpc-reflection=false"},
		map[string]string{
			"GRPC_ENABLED":     "false",
			"GRPC_ADDRESS":     ":9090",
			"GRPC_MAX_RECV_MB": "8",
			"GRPC_MAX_SEND_MB": "8",
			"GRPC_REFLECTION":  "true",
		},
	)
	if cfg.GRPCEnabled != true {
		t.Fatalf("GRPCEnabled=%v, want true (flag)", cfg.GRPCEnabled)
	}
	if cfg.GRPCAddress != ":5051" {
		t.Fatalf("GRPCAddress=%q, want %q (flag)", cfg.GRPCAddress, ":5051")
	}
	if cfg.GRPCMaxRecvMB != 16 || cfg.GRPCMaxSendMB != 32 {
		t.Fatalf("sizes=%d/%d, want 16/32 (flags)", cfg.GRPCMaxRecvMB, cfg.GRPCMaxSendMB)
	}
	if cfg.GRPCReflection != false {
		t.Fatalf("GRPCReflection=%v, want false (flag)", cfg.GRPCReflection)
	}
}

func TestLoadConfig_GRPC_EnvOnly_NoFlags(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{},
		map[string]string{
			"GRPC_ENABLED":     "true",
			"GRPC_ADDRESS":     "0.0.0.0:9090",
			"GRPC_MAX_RECV_MB": "64",
			"GRPC_MAX_SEND_MB": "64",
			"GRPC_REFLECTION":  "false",
		},
	)
	if cfg.GRPCEnabled != true {
		t.Fatalf("GRPCEnabled=%v, want true (env)", cfg.GRPCEnabled)
	}
	if cfg.GRPCAddress != "0.0.0.0:9090" {
		t.Fatalf("GRPCAddress=%q, want %q (env)", cfg.GRPCAddress, "0.0.0.0:9090")
	}
	if cfg.GRPCMaxRecvMB != 64 || cfg.GRPCMaxSendMB != 64 {
		t.Fatalf("sizes=%d/%d, want 64/64 (env)", cfg.GRPCMaxRecvMB, cfg.GRPCMaxSendMB)
	}
	if cfg.GRPCReflection != false {
		t.Fatalf("GRPCReflection=%v, want false (env)", cfg.GRPCReflection)
	}
}

func TestLoadConfig_TrustedSubnet_FlagOverridesEnv(t *testing.T) {
	cfg := runLoadConfigWithArgs(t,
		[]string{"-t", "192.168.1.0/24"},
		map[string]string{"TRUSTED_SUBNET": "10.0.0.0/8"},
	)
	if cfg.TrustedSubnet != "192.168.1.0/24" {
		t.Fatalf("TrustedSubnet=%q, want %q (flag override)", cfg.TrustedSubnet, "192.168.1.0/24")
	}
}
