package probe

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/omarshaarawi/loadbalancer/pkg/loadbalancer"
)

type Prober struct {
	client  *http.Client
	logger  *slog.Logger
	timeout time.Duration
}

func NewProber(timeout time.Duration, logger *slog.Logger) *Prober {
	return &Prober{
		client: &http.Client{
			Timeout: timeout,
		},
		logger:  logger,
		timeout: timeout,
	}
}

type ProbeResult struct {
	ServerID  string
	RIF       int32
	Latency   int64
	IsHealthy bool
	Error     error
	Timestamp time.Time
}

func (p *Prober) ProbeServer(ctx context.Context, server *loadbalancer.Server) *ProbeResult {
	result := &ProbeResult{
		ServerID:  server.ID,
		Timestamp: time.Now(),
		IsHealthy: false,
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://"+server.Address+"/health", nil)
	if err != nil {
		p.logger.Error("failed to create probe request",
			slog.String("server", server.ID),
			slog.String("error", err.Error()))
		result.Error = err
		return result
	}

	start := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Error("probe request failed",
			slog.String("server", server.ID),
			slog.String("error", err.Error()))
		result.Error = err
		return result
	}
	defer resp.Body.Close()

	result.Latency = time.Since(start).Milliseconds()

	if rifStr := resp.Header.Get("X-Requests-In-Flight"); rifStr != "" {
		var rif int32
		if _, err := fmt.Sscanf(rifStr, "%d", &rif); err == nil {
			result.RIF = rif
		}
	}

	result.IsHealthy = resp.StatusCode == http.StatusOK
	return result
}

func (p *Prober) StartProbing(servers []*loadbalancer.Server, interval time.Duration) chan *ProbeResult {
	results := make(chan *ProbeResult, len(servers))

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			for _, server := range servers {
				go func(srv *loadbalancer.Server) {
					ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
					defer cancel()

					result := p.ProbeServer(ctx, srv)
					results <- result
				}(server)
			}
		}
	}()

	return results
}
