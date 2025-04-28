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

func main() {
	addr := getServerAddress()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	runner, ctx := errgroup.WithContext(ctx)

	config, err := cfg.NewConfig()
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

	server := server.NewServer(addr, metrics)
	server.Run(ctx, runner)

	runner.Go(func() error {
		<-ctx.Done()
		db.GetInstance().Close()
		return server.Shutdown(ctx)
	})

	runner.Wait()
}

func getServerAddress() string {
	addr := flag.String("a", "localhost:8080", "HTTP server endpoint address")
	flag.Parse()

	if flag.NArg() > 0 {
		log.Printf("Error: unknown flags or arguments: %v\n", flag.Args())
		flag.Usage()
		os.Exit(1)
	}

	return *addr
}
