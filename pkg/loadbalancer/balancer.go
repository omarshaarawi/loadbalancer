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
	rrIndex   uint32
}

func NewLoadBalancer(config *Config, logger *slog.Logger) *LoadBalancer {
	if config == nil {
		config = &Config{
			ProbeInterval:    time.Second,
			ProbeTimeout:     time.Second * 2,
			HealthCheckPath:  "/health",
			SelectionChoices: 2,
			Algorithm:        AlgorithmPrequal,
			QRIF:             0.84,
		}
	}
	if config.Algorithm == "" {
		config.Algorithm = AlgorithmPrequal
	}
	if config.QRIF == 0 {
		config.QRIF = 0.84
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

			algorithm := string(lb.config.Algorithm)
			if result.IsHealthy {
				lb.metrics.serverHealth.WithLabelValues(srv.ID, algorithm).Set(1)
			} else {
				lb.metrics.serverHealth.WithLabelValues(srv.ID, algorithm).Set(0)
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
	if lb.config.Algorithm == AlgorithmRoundRobin {
		return lb.selectServerRR()
	}
	return lb.selectServerPrequal()
}

func (lb *LoadBalancer) selectServerRR() *Server {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	if len(lb.servers) == 0 {
		return nil
	}

	healthyServers := make([]*Server, 0, len(lb.servers))
	for _, server := range lb.servers {
		if server.IsHealthy {
			healthyServers = append(healthyServers, server)
		}
	}

	if len(healthyServers) == 0 {
		return nil
	}

	index := atomic.AddUint32(&lb.rrIndex, 1)
	return healthyServers[int(index-1)%len(healthyServers)]
}

func (lb *LoadBalancer) selectServerPrequal() *Server {
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
	healthyCandidates := make([]*Server, 0, len(candidates))
	for _, server := range candidates {
		if server.IsHealthy {
			healthyCandidates = append(healthyCandidates, server)
		}
	}

	if len(healthyCandidates) == 0 {
		return nil
	}

	rifThreshold := lb.calculateRIFThreshold(healthyCandidates)

	var coldServers []*Server
	var hotServers []*Server

	for _, server := range healthyCandidates {
		rif := atomic.LoadInt32(&server.RIF)
		if rif > rifThreshold {
			hotServers = append(hotServers, server)
		} else {
			coldServers = append(coldServers, server)
		}
	}

	if len(coldServers) > 0 {
		return lb.selectLowestLatency(coldServers)
	}

	return lb.selectLowestRIF(hotServers)
}

func (lb *LoadBalancer) calculateRIFThreshold(servers []*Server) int32 {
	if len(servers) == 0 {
		return 0
	}

	rifValues := make([]int32, len(servers))
	for i, server := range servers {
		rifValues[i] = atomic.LoadInt32(&server.RIF)
	}

	for i := 0; i < len(rifValues)-1; i++ {
		for j := i + 1; j < len(rifValues); j++ {
			if rifValues[i] > rifValues[j] {
				rifValues[i], rifValues[j] = rifValues[j], rifValues[i]
			}
		}
	}

	index := int(float64(len(rifValues)-1) * lb.config.QRIF)
	if index >= len(rifValues) {
		index = len(rifValues) - 1
	}

	return rifValues[index]
}

func (lb *LoadBalancer) selectLowestLatency(servers []*Server) *Server {
	if len(servers) == 0 {
		return nil
	}

	best := servers[0]
	minLatency := best.Latency

	for _, server := range servers[1:] {
		if server.Latency < minLatency {
			minLatency = server.Latency
			best = server
		}
	}

	return best
}

func (lb *LoadBalancer) selectLowestRIF(servers []*Server) *Server {
	if len(servers) == 0 {
		return nil
	}

	best := servers[0]
	minRIF := atomic.LoadInt32(&best.RIF)

	for _, server := range servers[1:] {
		rif := atomic.LoadInt32(&server.RIF)
		if rif < minRIF {
			minRIF = rif
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

	algorithm := string(lb.config.Algorithm)
	lb.metrics.requestDuration.WithLabelValues(algorithm).Observe(duration.Seconds())
	atomic.AddUint64(&lb.stats.SuccessfulRequests, 1)
}

func (lb *LoadBalancer) forwardRequest(server *Server, w http.ResponseWriter, r *http.Request) {
	algorithm := string(lb.config.Algorithm)
	atomic.AddInt32(&server.RIF, 1)
	lb.metrics.activeRequests.WithLabelValues(algorithm).Inc()

	defer func() {
		atomic.AddInt32(&server.RIF, -1)
		lb.metrics.activeRequests.WithLabelValues(algorithm).Dec()

		currentRIF := atomic.LoadInt32(&server.RIF)
		lb.metrics.serverRIF.WithLabelValues(server.ID, algorithm).Set(float64(currentRIF))
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
