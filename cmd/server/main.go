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
	flagDatabaseDSN     string
	flagHashKey         string
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
	flag.StringVar(&config.flagDatabaseDSN, "d", "", "database DSN")
	flag.StringVar(&config.flagHashKey, "k", "", "hash key")

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
	if config.flagDatabaseDSN != "" {
		config.DatabaseDSN = config.flagDatabaseDSN
	}
	if config.flagHashKey != "" {
		config.HashKey = config.flagHashKey
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

	logger, err := logger.Initialize(config.Logger)
	if err != nil {
		log.Fatal(err, "Init logger")
	}
	ctx = logger.Zerolog().WithContext(ctx)

	var storage db.MetricsStorage

	if config.DatabaseDSN != "" {
		storage = db.NewPostgresStore(config.DatabaseDSN)
		logger.Zerolog().Info().Msg("Using PostgreSQL storage")
	} else {
		fileStore := db.NewStore(config.FileStoragePath, config.StoreInterval)
		if config.Restore {
			if err := fileStore.LoadFromFile(); err != nil {
				logger.Zerolog().Error().Err(err).Msg("Failed to load data from file")
			}
		}
		storage = fileStore
		logger.Zerolog().Info().Msg("Using file storage")
	}

	metrics := services.NewMetricsService(storage)
	server := server.NewServer(config.ServerAddress, config.HashKey, metrics)
	server.Run(ctx, runner)

	if config.DatabaseDSN == "" && config.StoreInterval > 0 {
		ticker := time.NewTicker(config.StoreInterval)
		runner.Go(func() error {
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := storage.(*db.Store).SaveToFile(); err != nil {
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
		if config.DatabaseDSN == "" {
			if err := storage.(*db.Store).SaveToFile(); err != nil {
				logger.Zerolog().Error().Err(err).Msg("Failed to save data to file on shutdown")
			}
		}
		if err := storage.Close(); err != nil {
			logger.Zerolog().Error().Err(err).Msg("Failed to close storage")
		}
		return server.Shutdown(ctx)
	})

	runner.Wait()
}
