package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Heidric/metrics.git/internal/cfg"
	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/server"
	"github.com/Heidric/metrics.git/internal/services"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	cfg.Config
	flagAddress         string
	flagFileStoragePath string
	flagStoreInterval   time.Duration
	flagRestore         bool
}

func loadConfig() (*Config, error) {
	baseCfg, err := cfg.NewConfig()
	if err != nil {
		return nil, err
	}

	config := &Config{Config: *baseCfg}

	flag.StringVar(&config.flagAddress, "a", "", "HTTP server endpoint address")
	flag.StringVar(&config.flagFileStoragePath, "f", "", "file storage path")
	flag.DurationVar(&config.flagStoreInterval, "i", 0, "store interval in seconds")
	flag.BoolVar(&config.flagRestore, "r", true, "restore data from file")

	flag.Parse()

	if config.flagAddress != "" {
		config.ServerAddress = config.flagAddress
	}
	if config.flagFileStoragePath != "" {
		config.FileStoragePath = config.flagFileStoragePath
	}
	if config.flagStoreInterval != 0 {
		config.StoreInterval = config.flagStoreInterval
	}
	if flag.Lookup("r") != nil && flag.Lookup("r").Value.String() != "" {
		config.Restore = config.flagRestore
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

	storage := db.NewStore(config.FileStoragePath, config.StoreInterval)

	if config.Restore {
		if err := storage.LoadFromFile(); err != nil {
			logger.Zerolog().Error().Err(err).Msg("Failed to load data from file")
		}
	}

	metrics := services.NewMetricsService(storage)
	server := server.NewServer(config.ServerAddress, metrics)
	server.Run(ctx, runner)

	if config.StoreInterval > 0 {
		ticker := time.NewTicker(config.StoreInterval)
		runner.Go(func() error {
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := storage.SaveToFile(); err != nil {
						logger.Zerolog().Error().Err(err).Msg("Failed to save data to file")
					}
				case <-ctx.Done():
					return nil
				}
			}
		})
	}

	runner.Go(func() error {
		<-ctx.Done()
		if err := storage.SaveToFile(); err != nil {
			logger.Zerolog().Error().Err(err).Msg("Failed to save data to file on shutdown")
		}
		storage.Close()
		return server.Shutdown(ctx)
	})

	runner.Wait()
}
