package cfg

import (
	"os"
	"strconv"
	"time"

	"github.com/Heidric/metrics.git/pkg/log"
	"github.com/joho/godotenv"
)

type Config struct {
	Logger          *log.Config
	ServerAddress   string
	PollInterval    time.Duration
	ReportInterval  time.Duration
	StoreInterval   time.Duration
	FileStoragePath string
	Restore         bool
	DatabaseDSN     string
	HashKey         string
}

func NewConfig() (*Config, error) {
	godotenv.Load()

	config := &Config{
		Logger: &log.Config{},
	}

	config.ServerAddress = getEnv("ADDRESS", "localhost:8080")
	config.PollInterval = parseDuration("POLL_INTERVAL", 2*time.Second)
	config.ReportInterval = parseDuration("REPORT_INTERVAL", 10*time.Second)
	config.StoreInterval = parseDuration("STORE_INTERVAL", 300*time.Second)
	config.FileStoragePath = getEnv("FILE_STORAGE_PATH", "/tmp/metrics-db.json")
	config.Restore = parseBool("RESTORE", true)
	config.DatabaseDSN = getEnv("DATABASE_DSN", "")
	config.HashKey = getEnv("HASH_KEY", "")

	config.Logger.SetDefault()
	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func parseDuration(key string, defaultValue time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if sec, err := strconv.Atoi(value); err == nil {
			return time.Duration(sec) * time.Second
		}
		if dur, err := time.ParseDuration(value); err == nil {
			return dur
		}
	}
	return defaultValue
}

func parseBool(key string, defaultValue bool) bool {
	if value, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}
