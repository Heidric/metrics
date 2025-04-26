package main

import (
	"fmt"
	"net/http"

	"github.com/Heidric/metrics.git/internal/db"
	"github.com/Heidric/metrics.git/internal/handlers"
	"github.com/Heidric/metrics.git/internal/services"
)

func main() {
	addr := "localhost:8090"

	storage := db.GetInstance()
	service := services.NewMetricsService(storage)
	MetricHandlers := handlers.NewMetricsHandlers(service)

	mux := http.NewServeMux()

	mux.HandleFunc("/update/", MetricHandlers.UpdateMetricHandler)

	mux.HandleFunc(`/`, MetricHandlers.NotFoundHandler)

	fmt.Println("Starting server on ", addr)

	if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
