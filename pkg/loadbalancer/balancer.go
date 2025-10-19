package loadbalancer

import (
	"context"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type LoadBalancer struct {
	servers   []*Server
	probePool map[string]*ProbeResult
	config    *Config
	stats     *Stats
	logger    *slog.Logger
	metrics   *Metrics
	mutex     sync.RWMutex
}

func NewLoadBalancer(config *Config, logger *slog.Logger) *LoadBalancer {
	if config == nil {
		config = &Config{
			ProbeInterval:    time.Second,
			ProbeTimeout:     time.Second * 2,
			HealthCheckPath:  "/health",
			SelectionChoices: 2,
		}
	}

	return &LoadBalancer{
		servers:   make([]*Server, 0),
		probePool: make(map[string]*ProbeResult),
		config:    config,
		stats:     &Stats{},
		logger:    logger,
		metrics:   NewMetrics(),
	}
}

func (lb *LoadBalancer) StartProbing() {
	go func() {
		ticker := time.NewTicker(lb.config.ProbeInterval)
		defer ticker.Stop()

		for range ticker.C {
			lb.probeAllServers()
		}
	}()
}

func (lb *LoadBalancer) probeAllServers() {
	lb.mutex.RLock()
	servers := make([]*Server, len(lb.servers))
	copy(servers, lb.servers)
	lb.mutex.RUnlock()

	for _, server := range servers {
		go func(srv *Server) {
			result := lb.probeServer(srv)

			lb.mutex.Lock()
			lb.probePool[srv.ID] = result
			srv.IsHealthy = result.IsHealthy
			srv.Latency = result.Latency
			lb.mutex.Unlock()

			if result.IsHealthy {
				lb.metrics.serverHealth.WithLabelValues(srv.ID).Set(1)
			} else {
				lb.metrics.serverHealth.WithLabelValues(srv.ID).Set(0)
			}
		}(server)
	}
}

func (lb *LoadBalancer) probeServer(server *Server) *ProbeResult {
	ctx, cancel := context.WithTimeout(context.Background(), lb.config.ProbeTimeout)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET",
		"http://"+server.Address+lb.config.HealthCheckPath, nil)
	if err != nil {
		lb.logger.Error("Failed to create probe request",
			slog.String("server", server.ID),
			slog.String("error", err.Error()))
		return &ProbeResult{
			Timestamp: time.Now(),
			IsHealthy: false,
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		lb.logger.Error("Probe request failed",
			slog.String("server", server.ID),
			slog.String("error", err.Error()))
		return &ProbeResult{
			Timestamp: time.Now(),
			IsHealthy: false,
		}
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	return &ProbeResult{
		Timestamp: time.Now(),
		RIF:       atomic.LoadInt32(&server.RIF),
		Latency:   duration.Milliseconds(),
		IsHealthy: resp.StatusCode == http.StatusOK,
	}
}

func (lb *LoadBalancer) AddServer(server *Server) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	lb.servers = append(lb.servers, server)
}

func (lb *LoadBalancer) SelectServer() *Server {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	if len(lb.servers) == 0 {
		return nil
	}

	candidates := make([]*Server, 0, lb.config.SelectionChoices)
	for i := 0; i < lb.config.SelectionChoices; i++ {
		randomIndex := rand.Intn(len(lb.servers))
		candidates = append(candidates, lb.servers[randomIndex])
	}

	return lb.selectBestCandidate(candidates)
}

func (lb *LoadBalancer) selectBestCandidate(candidates []*Server) *Server {
	var best *Server
	var bestScore float64 = float64(^uint64(0) >> 1)

	for _, server := range candidates {
		if !server.IsHealthy {
			continue
		}

		score := float64(server.RIF) * float64(server.Latency)
		if score < bestScore {
			bestScore = score
			best = server
		}
	}

	return best
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddUint64(&lb.stats.TotalRequests, 1)

	server := lb.SelectServer()
	if server == nil {
		lb.logger.Error("No available servers")
		atomic.AddUint64(&lb.stats.FailedRequests, 1)
		http.Error(w, "No available servers", http.StatusServiceUnavailable)
		return
	}

	start := time.Now()
	lb.forwardRequest(server, w, r)
	duration := time.Since(start)

	lb.metrics.requestDuration.Observe(duration.Seconds())
	atomic.AddUint64(&lb.stats.SuccessfulRequests, 1)
}

func (lb *LoadBalancer) forwardRequest(server *Server, w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&server.RIF, 1)
	lb.metrics.activeRequests.Inc()

	defer func() {
		atomic.AddInt32(&server.RIF, -1)
		lb.metrics.activeRequests.Dec()

		currentRIF := atomic.LoadInt32(&server.RIF)
		lb.metrics.serverRIF.WithLabelValues(server.ID).Set(float64(currentRIF))
	}()

	targetURL, _ := url.Parse("http://" + server.Address)
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		lb.logger.Error("Proxy error", slog.String("error", err.Error()))
		atomic.AddUint64(&lb.stats.FailedRequests, 1)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	}

	proxy.ServeHTTP(w, r)
}
