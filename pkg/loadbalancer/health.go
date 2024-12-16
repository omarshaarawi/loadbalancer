package loadbalancer

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type HealthChecker struct {
	mutex     sync.RWMutex
	servers   map[string]*Server
	logger    *slog.Logger
	metrics   *Metrics
	threshold int
	timeout   time.Duration
}

func NewHealthChecker(logger *slog.Logger, metrics *Metrics) *HealthChecker {
	return &HealthChecker{
		servers:   make(map[string]*Server),
		logger:    logger,
		metrics:   metrics,
		threshold: 3,
		timeout:   time.Second * 2,
	}
}

type HealthStatus struct {
	IsHealthy           bool
	LastCheck           time.Time
	ConsecutiveFailures int
	LastError           error
}

func (hc *HealthChecker) AddServer(server *Server) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.servers[server.ID] = server
}

func (hc *HealthChecker) RemoveServer(serverID string) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	delete(hc.servers, serverID)
}

func (hc *HealthChecker) StartHealthChecks(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.checkAll(ctx)
		}
	}
}

func (hc *HealthChecker) checkAll(ctx context.Context) {
	hc.mutex.RLock()
	servers := make([]*Server, 0, len(hc.servers))
	for _, server := range hc.servers {
		servers = append(servers, server)
	}
	hc.mutex.RUnlock()

	for _, server := range servers {
		go func(srv *Server) {
			status := hc.checkHealth(ctx, srv)

			hc.mutex.Lock()
			srv.IsHealthy = status.IsHealthy
			srv.LastProbe = status.LastCheck
			hc.mutex.Unlock()

			if status.IsHealthy {
				hc.metrics.serverHealth.WithLabelValues(srv.ID).Set(1)
			} else {
				hc.metrics.serverHealth.WithLabelValues(srv.ID).Set(0)
			}

			if !status.IsHealthy {
				hc.logger.Warn("Server unhealthy",
					slog.String("server_id", srv.ID),
					slog.String("error", status.LastError.Error()))
			}
		}(server)
	}
}

func (hc *HealthChecker) checkHealth(ctx context.Context, server *Server) *HealthStatus {
	ctx, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()

	status := &HealthStatus{
		LastCheck: time.Now(),
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://"+server.Address+"/health", nil)
	if err != nil {
		status.LastError = err
		return status
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		status.LastError = err
		return status
	}
	defer resp.Body.Close()

	status.IsHealthy = resp.StatusCode == http.StatusOK
	return status
}
