package unit

import (
	"log/slog"
	"testing"
	"time"

	"github.com/omarshaarawi/loadbalancer/pkg/loadbalancer"
)

func TestLoadBalancer(t *testing.T) {
	logger := slog.Default()
	config := &loadbalancer.Config{
		ProbeInterval:    time.Second,
		ProbeTimeout:     time.Second * 2,
		HealthCheckPath:  "/health",
		SelectionChoices: 2,
	}

	lb := loadbalancer.NewLoadBalancer(config, logger)

	server1 := &loadbalancer.Server{
		ID:        "test1",
		Address:   "localhost:8081",
		IsHealthy: true,
	}
	server2 := &loadbalancer.Server{
		ID:        "test2",
		Address:   "localhost:8082",
		IsHealthy: true,
	}

	lb.AddServer(server1)
	lb.AddServer(server2)

	selected := lb.SelectServer()
	if selected == nil {
		t.Error("Expected server to be selected, got nil")
	}
}
