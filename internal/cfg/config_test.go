package cfg

import (
	"testing"
	"time"
)

func Test_getEnv_DefaultAndSet(t *testing.T) {
	t.Run("returns default when not set", func(t *testing.T) {
		const key = "CFG_TEST_ENV_1"
		t.Setenv(key, "")
		got := getEnv(key, "fallback")
		if got != "fallback" {
			t.Fatalf("getEnv default = %q, want %q", got, "fallback")
		}
	})

	t.Run("returns set value", func(t *testing.T) {
		const key = "CFG_TEST_ENV_2"
		t.Setenv(key, "value")
		got := getEnv(key, "fallback")
		if got != "value" {
			t.Fatalf("getEnv set = %q, want %q", got, "value")
		}
	})
}

func Test_parseDuration(t *testing.T) {
	const key = "CFG_TEST_DURATION"

	t.Run("valid integer seconds", func(t *testing.T) {
		t.Setenv(key, "7")
		got := parseDuration(key, 42*time.Second)
		if got != 7*time.Second {
			t.Fatalf("parseDuration(valid) = %v, want %v", got, 7*time.Second)
		}
	})

	t.Run("invalid integer -> default", func(t *testing.T) {
		t.Setenv(key, "not-an-int")
		got := parseDuration(key, 5*time.Second)
		if got != 5*time.Second {
			t.Fatalf("parseDuration(invalid) = %v, want %v", got, 5*time.Second)
		}
	})

	t.Run("unset -> default", func(t *testing.T) {
		t.Setenv(key, "")
		got := parseDuration(key, 3*time.Second)
		if got != 3*time.Second {
			t.Fatalf("parseDuration(unset) = %v, want %v", got, 3*time.Second)
		}
	})
}

func Test_parseBool(t *testing.T) {
	const key = "CFG_TEST_BOOL"

	t.Run("true", func(t *testing.T) {
		t.Setenv(key, "true")
		if got := parseBool(key, false); got != true {
			t.Fatalf("parseBool(true) = %v, want true", got)
		}
	})

	t.Run("false", func(t *testing.T) {
		t.Setenv(key, "false")
		if got := parseBool(key, true); got != false {
			t.Fatalf("parseBool(false) = %v, want false", got)
		}
	})

	t.Run("invalid -> default", func(t *testing.T) {
		t.Setenv(key, "definitely-not-bool")
		if got := parseBool(key, true); got != true {
			t.Fatalf("parseBool(invalid) = %v, want default true", got)
		}
	})

	t.Run("unset -> default", func(t *testing.T) {
		t.Setenv(key, "")
		if got := parseBool(key, false); got != false {
			t.Fatalf("parseBool(unset) = %v, want default false", got)
		}
	})
}

func Test_NewConfig_Defaults(t *testing.T) {
	t.Setenv("ADDRESS", "")
	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("REPORT_INTERVAL", "")
	t.Setenv("STORE_INTERVAL", "")
	t.Setenv("STORE_FILE", "")
	t.Setenv("RESTORE", "")
	t.Setenv("DATABASE_DSN", "")
	t.Setenv("HASH_KEY", "")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:8080" {
		t.Fatalf("ServerAddress = %q, want %q", cfg.ServerAddress, "localhost:8080")
	}
	if cfg.PollInterval != 2*time.Second {
		t.Fatalf("PollInterval = %v, want %v", cfg.PollInterval, 2*time.Second)
	}
	if cfg.ReportInterval != 10*time.Second {
		t.Fatalf("ReportInterval = %v, want %v", cfg.ReportInterval, 10*time.Second)
	}
	if cfg.StoreInterval != 300*time.Second {
		t.Fatalf("StoreInterval = %v, want %v", cfg.StoreInterval, 300*time.Second)
	}
	if cfg.FileStoragePath != "/tmp/metrics-db.json" {
		t.Fatalf("FileStoragePath = %q, want %q", cfg.FileStoragePath, "/tmp/metrics-db.json")
	}
	if cfg.Restore != true {
		t.Fatalf("Restore = %v, want %v", cfg.Restore, true)
	}
	if cfg.DatabaseDSN != "" {
		t.Fatalf("DatabaseDSN = %q, want empty", cfg.DatabaseDSN)
	}
	if cfg.HashKey != "" {
		t.Fatalf("HashKey = %q, want empty", cfg.HashKey)
	}
	if cfg.Logger == nil {
		t.Fatalf("Logger should be initialized")
	}
}

func Test_NewConfig_FromEnv(t *testing.T) {
	t.Setenv("ADDRESS", "0.0.0.0:9000")
	t.Setenv("POLL_INTERVAL", "3")
	t.Setenv("REPORT_INTERVAL", "17")
	t.Setenv("STORE_INTERVAL", "600")
	t.Setenv("STORE_FILE", "/var/lib/metrics.json")
	t.Setenv("RESTORE", "false")
	t.Setenv("DATABASE_DSN", "postgres://u:p@h:5432/db?sslmode=disable")
	t.Setenv("HASH_KEY", "supersecret")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	if cfg.ServerAddress != "0.0.0.0:9000" {
		t.Fatalf("ServerAddress = %q", cfg.ServerAddress)
	}
	if cfg.PollInterval != 3*time.Second {
		t.Fatalf("PollInterval = %v", cfg.PollInterval)
	}
	if cfg.ReportInterval != 17*time.Second {
		t.Fatalf("ReportInterval = %v", cfg.ReportInterval)
	}
	if cfg.StoreInterval != 600*time.Second {
		t.Fatalf("StoreInterval = %v", cfg.StoreInterval)
	}
	if cfg.FileStoragePath != "/var/lib/metrics.json" {
		t.Fatalf("FileStoragePath = %q", cfg.FileStoragePath)
	}
	if cfg.Restore != false {
		t.Fatalf("Restore = %v", cfg.Restore)
	}
	if cfg.DatabaseDSN != "postgres://u:p@h:5432/db?sslmode=disable" {
		t.Fatalf("DatabaseDSN = %q", cfg.DatabaseDSN)
	}
	if cfg.HashKey != "supersecret" {
		t.Fatalf("HashKey = %q", cfg.HashKey)
	}
}
