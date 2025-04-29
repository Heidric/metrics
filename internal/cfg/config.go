package cfg

import (
	"os"
	"strconv"
	"time"

	"github.com/Heidric/metrics.git/pkg/log"
	"github.com/joho/godotenv"
	"github.com/vrischmann/envconfig"
)

type Config struct {
	Logger         *log.Config
	ServerAddress  string        `envconfig:"ADDRESS"`
	PollInterval   time.Duration `envconfig:"POLL_INTERVAL"`
	ReportInterval time.Duration `envconfig:"REPORT_INTERVAL"`
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load()

	config := &Config{
		Logger: &log.Config{},
	}

	defaults := map[string]string{
		"ADDRESS":         "localhost:8080",
		"POLL_INTERVAL":   "2s",
		"REPORT_INTERVAL": "10s",
	}

	for key, value := range defaults {
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	if val := os.Getenv("POLL_INTERVAL"); val != "" {
		if sec, err := strconv.Atoi(val); err == nil {
			os.Setenv("POLL_INTERVAL", strconv.Itoa(sec)+"s")
		}
	}
	if val := os.Getenv("REPORT_INTERVAL"); val != "" {
		if sec, err := strconv.Atoi(val); err == nil {
			os.Setenv("REPORT_INTERVAL", strconv.Itoa(sec)+"s")
		}
	}

	if err := envconfig.Init(config); err != nil {
		return nil, err
	}

	config.Logger.SetDefault()

	return config, nil
}
