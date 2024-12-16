package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requestDuration prometheus.Histogram
	activeRequests  prometheus.Gauge
	serverHealth    *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		requestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Time spent processing request",
			Buckets: prometheus.DefBuckets,
		}),
		activeRequests: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "active_requests",
			Help: "Number of requests currently being processed",
		}),
		serverHealth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "server_health",
				Help: "Health status of servers",
			},
			[]string{"server_id"},
		),
	}

	prometheus.MustRegister(m.requestDuration)
	prometheus.MustRegister(m.activeRequests)
	prometheus.MustRegister(m.serverHealth)

	return m
}
