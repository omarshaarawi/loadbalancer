package loadbalancer

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requestDuration *prometheus.HistogramVec
	activeRequests  *prometheus.GaugeVec
	serverHealth    *prometheus.GaugeVec
	serverRIF       *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "request_duration_seconds",
				Help:    "Time spent processing request",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"algorithm"},
		),
		activeRequests: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "active_requests",
				Help: "Number of requests currently being processed",
			},
			[]string{"algorithm"},
		),
		serverHealth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "server_health",
				Help: "Health status of servers",
			},
			[]string{"server_id", "algorithm"},
		),
		serverRIF: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "server_rif",
				Help: "Requests in flight per server",
			},
			[]string{"server_id", "algorithm"},
		),
	}

	prometheus.MustRegister(m.requestDuration)
	prometheus.MustRegister(m.activeRequests)
	prometheus.MustRegister(m.serverHealth)
	prometheus.MustRegister(m.serverRIF)

	return m
}
