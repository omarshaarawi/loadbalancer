package server

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/omarshaarawi/loadbalancer/internal/config"
	"github.com/omarshaarawi/loadbalancer/pkg/loadbalancer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	httpServer *http.Server
	lb         *loadbalancer.LoadBalancer
	config     *config.Config
	logger     *slog.Logger
	wg         sync.WaitGroup
}

func NewServer(cfg *config.Config, logger *slog.Logger) *Server {
	lb := loadbalancer.NewLoadBalancer(&loadbalancer.Config{
		ProbeInterval:    cfg.ProbeInterval,
		ProbeTimeout:     cfg.ProbeTimeout,
		HealthCheckPath:  cfg.HealthCheckPath,
		SelectionChoices: cfg.SelectionChoices,
	}, logger)

	for _, serverCfg := range cfg.Servers {
		lb.AddServer(&loadbalancer.Server{
			ID:        serverCfg.ID,
			Address:   serverCfg.Address,
			IsHealthy: true,
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/", lb)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", handleHealth)

	return &Server{
		httpServer: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      mux,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
		lb:     lb,
		config: cfg,
		logger: logger,
	}
}

func (s *Server) Start() error {
	s.logger.Info("Starting server", slog.String("port", s.config.Port))

	s.lb.StartProbing()

	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return err
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy"}`))
}
