package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Heidric/metrics.git/internal/cfg"
	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/server"
	"github.com/Heidric/metrics.git/internal/services"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	cfg.Config
	flagAddress string
}

func loadConfig() (*Config, error) {
	baseCfg, err := cfg.NewConfig()
	if err != nil {
		return nil, err
	}

	config := &Config{Config: *baseCfg}

	flag.StringVar(&config.flagAddress, "a", "", "HTTP server endpoint address")
	flag.Parse()

	if config.flagAddress != "" {
		config.ServerAddress = config.flagAddress
	}

	return config, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	runner, ctx := errgroup.WithContext(ctx)

	config, err := loadConfig()
	if err != nil {
		log.Fatal(err, "Load config")
	}

	logger, err := logger.IniInitialize(config.Logger)
	if err != nil {
		log.Fatal(err, "Init logger")
	}
	ctx = logger.Zerolog().WithContext(ctx)

	storage := db.GetInstance()
	metrics := services.NewMetricsService(storage)

	server := server.NewServer(config.ServerAddress, metrics)
	server.Run(ctx, runner)

	runner.Go(func() error {
		<-ctx.Done()
		db.GetInstance().Close()
		return server.Shutdown(ctx)
	})

	runner.Wait()
}
