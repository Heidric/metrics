package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Heidric/metrics.git/internal/buildinfo"
	"github.com/Heidric/metrics.git/internal/cfg"
	intcrypto "github.com/Heidric/metrics.git/internal/crypto"
	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/logger"
	"github.com/Heidric/metrics.git/internal/server"
	"github.com/Heidric/metrics.git/internal/services"
	"golang.org/x/sync/errgroup"
)

// Config aggregates runtime configuration for the server command.
// It embeds cfg.Config (shared service settings) and carries CLI flag values
// parsed in main before they are merged into the final runtime config.
type Config struct {
	flagAddress         string // listen address from flag (e.g., ":8080")
	flagFileStoragePath string // path to JSON file for on-disk persistence
	flagDatabaseDSN     string // PostgreSQL DSN; when set, enables DB-backed storage
	flagHashKey         string // HMAC key used by hash middleware and related logic
	flagCryptoKey       string // path to RSA private key (PEM)

	cfg.Config

	flagStoreInterval time.Duration // interval for periodic persistence; 0 => sync on each update

	flagRestore bool // restore state from file on startup
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
	flag.StringVar(&config.flagCryptoKey, "crypto-key", "", "path to RSA private key (PEM)")
	flag.String("config", "", "path to JSON config file")
	flag.String("c", "", "path to JSON config file (shorthand)")

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
	var restoreSet bool
	flag.CommandLine.Visit(func(f *flag.Flag) {
		if f.Name == "r" {
			restoreSet = true
		}
	})
	if restoreSet {
		config.Restore = config.flagRestore
	}
	if config.flagDatabaseDSN != "" {
		config.DatabaseDSN = config.flagDatabaseDSN
	}
	if config.flagHashKey != "" {
		config.HashKey = config.flagHashKey
	}
	if config.flagCryptoKey != "" {
		config.CryptoKeyPath = config.flagCryptoKey
	}

	return config, nil
}

func main() {
	buildinfo.PrintStdout()

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

	var opts []server.Option
	if config.CryptoKeyPath != "" {
		pkey, err := intcrypto.ParseRSAPrivateKeyPEM(config.CryptoKeyPath)
		if err != nil {
			logger.Zerolog().Error().Err(err).Msg("failed to load RSA private key")
		}
		opts = append(opts, server.WithPrivateKey(pkey))
	}

	metrics := services.NewMetricsService(storage)
	server := server.NewServer(config.ServerAddress, config.HashKey, metrics, opts...)
	server.Run(ctx, runner)
}
