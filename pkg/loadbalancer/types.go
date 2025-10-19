package loadbalancer

import (
	"sync"
	"time"
)

type Server struct {
	ID        string
	Address   string
	RIF       int32
	Latency   int64
	IsHealthy bool
	LastProbe time.Time
}

type ProbeResult struct {
	Timestamp time.Time
	RIF       int32
	Latency   int64
	IsHealthy bool
}

type Algorithm string

const (
	AlgorithmPrequal     Algorithm = "prequal"
	AlgorithmRoundRobin  Algorithm = "roundrobin"
)

type Config struct {
	ProbeInterval    time.Duration
	ProbeTimeout     time.Duration
	HealthCheckPath  string
	SelectionChoices int
	Algorithm        Algorithm
	QRIF             float64
}

type Stats struct {
	TotalRequests      uint64
	SuccessfulRequests uint64
	FailedRequests     uint64
	AverageLatency     float64
	mutex              sync.RWMutex
}
