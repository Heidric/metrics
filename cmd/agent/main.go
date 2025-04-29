package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Heidric/metrics.git/internal/cfg"
)

type Config struct {
	cfg.Config
	flagAddress string
	flagPoll    int
	flagReport  int
}

func loadConfig() (*Config, error) {
	baseCfg, err := cfg.NewConfig()
	if err != nil {
		return nil, err
	}

	config := &Config{Config: *baseCfg}

	flag.StringVar(&config.flagAddress, "a", "", "HTTP server endpoint address")
	flag.IntVar(&config.flagPoll, "p", 0, "Poll interval in seconds")
	flag.IntVar(&config.flagReport, "r", 0, "Report interval in seconds")
	flag.Parse()

	if config.flagAddress != "" {
		config.ServerAddress = config.flagAddress
	}
	if config.flagPoll > 0 {
		config.PollInterval = time.Duration(config.flagPoll) * time.Second
	}
	if config.flagReport > 0 {
		config.ReportInterval = time.Duration(config.flagReport) * time.Second
	}

	return config, nil
}

func main() {
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting agent with configuration:\n")
	fmt.Printf("  Server Address: %s\n", config.ServerAddress)
	fmt.Printf("  Poll Interval: %v\n", config.PollInterval)
	fmt.Printf("  Report Interval: %v\n", config.ReportInterval)
}
