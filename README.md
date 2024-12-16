# Go Load Balancer


This project is an implementation of the load balancing algorithm described in the paper "Load is not what you should balance: Introducing Prequal" (NSDI '24). It demonstrates key concepts
including Power of d choices, RIF (Requests in Flight) tracking, and HCL (Hot-Cold Lexicographic) scoring.

## What's inside

- Power of d choices with HCL (Hot-Cold Lexicographic) scoring
- RIF tracking to see what servers are actually busy
- Health checks that run in the background
- Prometheus metrics and Grafana dashboards for visibility

## Prerequisites

- Go 1.23+
- Docker
- Docker Compose

## Getting started

```bash
git clone https://github.com/yourusername/loadbalancer.git
cd loadbalancer
docker-compose up --build
```

Then check out:
- Load Balancer at http://localhost:8080
- Prometheus at http://localhost:9090
- Grafana at http://localhost:3001 (login: admin/admin)

## How it works

**Server selection:** Pick d servers at random, score them using `RIF × Latency`, send the request to whoever has the lowest score.

**Health checks:** Background goroutine pings all servers on a timer. Dead servers get removed from rotation.

**Metrics:** We export Prometheus metrics for request duration, active connections, server health, and RIF counts.

## Testing it out

Quick test:
```bash
curl http://localhost:8080 ```

Load test with hey:
```bash
go install github.com/rakyll/hey@latest
hey -n 1000 -c 50 http://localhost:8080/
```

Or just loop curl if you want:
```bash
for i in {1..10}; do curl http://localhost:8080; done
```

## References

Based on the paper: [Load is not what you should balance: Introducing Prequal](https://www.usenix.org/conference/nsdi24/presentation/wydrowski) (NSDI '24)
