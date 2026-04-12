package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)


type Metrics struct {
	RequestsTotal *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "gateway",
				Name: "http_requests_total",
				Help: "Total HTTP requests",
			},
			[]string{"route", "method", "status_class"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "gateway",
				Name: "http_request_duration_seconds",
				Help: "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"route", "method", "status_class"},
		),
	}
}
