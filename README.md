# Go Load Balancer (Prequal Implementation)

This project is an implementation of the load balancing algorithm described in the paper "Load is not what you should balance: Introducing Prequal" (NSDI '24). It demonstrates key concepts including Power of d choices, RIF (Requests in Flight) tracking, and HCL (Hot-Cold Lexicographic) scoring.

## Features

- Power of d choices load balancing algorithm
- RIF (Requests in Flight) tracking
- HCL (Hot-Cold Lexicographic) scoring
- Asynchronous health checking and probing
- Prometheus metrics integration
- Grafana dashboards

## Prerequisites

- Go 1.23+
- Docker
- Docker Compose

## Quick Start

1. Clone the repository:
```bash
git clone https://github.com/yourusername/loadbalancer.git
cd loadbalancer
```

2. Start the services:
```bash
docker-compose up --build
```

3. Access the services:
- Load Balancer: http://localhost:8080
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001 (admin/admin)

## Architecture

The load balancer implements several key components:

### Server Selection
Uses the Power of d choices algorithm combined with HCL scoring:
```go
score := float64(server.RIF) * float64(server.Latency)
```

### Health Checking
Regular health checks with configurable intervals:
```go
func (lb *LoadBalancer) StartProbing() {
    go func() {
        ticker := time.NewTicker(lb.config.ProbeInterval)
        for range ticker.C {
            lb.probeAllServers()
        }
    }()
}
```

### Metrics
Prometheus metrics for monitoring:
- Request duration
- Active requests
- Server health
- RIF counts

### Load Testing
Using hey:
```bash
go install github.com/rakyll/hey@latest
hey -n 1000 -c 50 http://localhost:8080/
```

Using curl:
```bash
# Basic test
curl http://localhost:8080

# Multiple requests
for i in {1..10}; do curl http://localhost:8080; done
```

## Project Structure
```
.
├── cmd/
│   └── server/          # Main application
├── config/
│   ├── grafana/         # Grafana configuration
│   ├── nginx/           # NGINX configuration
│   └── prometheus/      # Prometheus configuration
├── internal/
│   ├── config/          # Configuration management
│   ├── metrics/         # Metrics collection
│   ├── probe/           # Health checking
│   └── server/          # Server implementation
├── pkg/
│   └── loadbalancer/    # Core load balancing logic
└── tests/
    └── unit/            # Unit tests
```

## References

- [Load is not what you should balance: Introducing Prequal](https://www.usenix.org/conference/nsdi24/presentation/wydrowski)
