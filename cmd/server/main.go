package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Heidric/metrics.git/internal/buildinfo"
	"github.com/Heidric/metrics.git/internal/cfg"
	intcrypto "github.com/Heidric/metrics.git/internal/crypto"
	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/grpcsvc"
	"github.com/Heidric/metrics.git/internal/logger"
	intpb "github.com/Heidric/metrics.git/internal/pb"
	"github.com/Heidric/metrics.git/internal/server"
	"github.com/Heidric/metrics.git/internal/services"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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
	flagTrustedSubnet   string // trusted subnet in CIDR

	flagGRPCEnabled    bool   // --grpc
	flagGRPCAddress    string // --grpc-addr
	flagGRPCMaxRecvMB  int    // --grpc-max-recv
	flagGRPCMaxSendMB  int    // --grpc-max-send
	flagGRPCReflection bool   // --grpc-reflection

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
	flag.StringVar(&config.flagTrustedSubnet, "t", "", "trusted subnet in CIDR (e.g. 10.0.0.0/8)")
	flag.BoolVar(&config.flagGRPCEnabled, "grpc", false, "enable gRPC server")
	flag.StringVar(&config.flagGRPCAddress, "grpc-addr", "", "gRPC listen address")
	flag.IntVar(&config.flagGRPCMaxRecvMB, "grpc-max-recv", 0, "gRPC max recv MB")
	flag.IntVar(&config.flagGRPCMaxSendMB, "grpc-max-send", 0, "gRPC max send MB")
	flag.BoolVar(&config.flagGRPCReflection, "grpc-reflection", true, "enable gRPC reflection")
	flag.String("config", "", "path to JSON config file")
	flag.String("c", "", "path to JSON config file (shorthand)")

	flag.Parse()

	visited := map[string]bool{}
	flag.CommandLine.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})

	if config.flagAddress != "" {
		config.ServerAddress = config.flagAddress
	}
	if config.flagFileStoragePath != "" {
		config.FileStoragePath = config.flagFileStoragePath
	}
	if config.flagStoreInterval != 0 {
		config.StoreInterval = config.flagStoreInterval
	}
	if visited["r"] {
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
	if config.flagTrustedSubnet != "" {
		config.TrustedSubnet = config.flagTrustedSubnet
	}

	if visited["grpc"] {
		config.GRPCEnabled = config.flagGRPCEnabled
	}
	if config.flagGRPCAddress != "" {
		config.GRPCAddress = config.flagGRPCAddress
	}
	if config.flagGRPCMaxRecvMB > 0 {
		config.GRPCMaxRecvMB = config.flagGRPCMaxRecvMB
	}
	if config.flagGRPCMaxSendMB > 0 {
		config.GRPCMaxSendMB = config.flagGRPCMaxSendMB
	}
	if visited["grpc-reflection"] {
		config.GRPCReflection = config.flagGRPCReflection
	}

	return config, nil
}

func main() {
	buildinfo.PrintStdout()

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
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
	var subnetOpt []server.Option
	if ts := strings.TrimSpace(config.TrustedSubnet); ts != "" {
		_, ipnet, err := net.ParseCIDR(ts)
		if err != nil {
			log.Fatalf("invalid TRUSTED_SUBNET/CIDR %q: %v", ts, err)
		}
		subnetOpt = append(subnetOpt, server.WithTrustedSubnet(ipnet))
	}

	srv := server.NewServer(config.ServerAddress, config.HashKey, metrics, append(opts, subnetOpt...)...)
	srv.Run(ctx, runner)

	var grpcSrv *grpc.Server
	var grpcLn net.Listener
	if config.GRPCEnabled {
		var trustedNet *net.IPNet
		if ts := strings.TrimSpace(config.TrustedSubnet); ts != "" {
			_, ipnet, err := net.ParseCIDR(ts)
			if err != nil {
				log.Fatalf("invalid TRUSTED_SUBNET for gRPC: %v", err)
			}
			trustedNet = ipnet
		}
		grpcSrv = grpcsvc.BuildServer(trustedNet, config.GRPCMaxRecvMB, config.GRPCMaxSendMB)
		svc := grpcsvc.New(metrics, trustedNet)
		intpb.RegisterMetricsServiceServer(grpcSrv, svc)
		if config.GRPCReflection {
			reflection.Register(grpcSrv)
		}
		ln, err := net.Listen("tcp", config.GRPCAddress)
		if err != nil {
			log.Fatalf("gRPC listen: %v", err)
		}
		grpcLn = ln
		go func() {
			if err := grpcSrv.Serve(ln); err != nil {
				log.Printf("gRPC serve error: %v", err)
			}
		}()
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Zerolog().Error().Err(err).Msg("server shutdown error")
	}

	if grpcSrv != nil {
		grpcSrv.GracefulStop()
		_ = grpcLn.Close()
	}

	if closer, ok := storage.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			logger.Zerolog().Error().Err(err).Msg("storage close error")
		}
	}

	if err := runner.Wait(); err != nil {
		logger.Zerolog().Error().Err(err).Msg("server background error")
	}
}
