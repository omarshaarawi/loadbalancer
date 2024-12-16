package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omarshaarawi/loadbalancer/pkg/loadbalancer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	LevelTrace = slog.Level(-8)
	LevelFatal = slog.Level(12)
)

func main() {
	ctx := context.Background()
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	config := &loadbalancer.Config{
		ProbeInterval:    time.Second,
		ProbeTimeout:     time.Second * 2,
		HealthCheckPath:  "/health",
		SelectionChoices: 2,
	}

	lb := loadbalancer.NewLoadBalancer(config, logger)

	testServers := []string{
		"server1:80",
		"server2:80",
		"server3:80",
	}

	for i, addr := range testServers {
		lb.AddServer(&loadbalancer.Server{
			ID:        fmt.Sprintf("server-%d", i),
			Address:   addr,
			IsHealthy: true,
		})
	}

	lb.StartProbing()

	mux := http.NewServeMux()
	mux.Handle("/", lb)
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    ":" + *port,
		Handler: mux,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Server shutdown error", slog.String("error", err.Error()))
		}
	}()

	logger.Info("Starting server on port " + *port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Log(ctx, LevelFatal, "Server error")
	}
}
