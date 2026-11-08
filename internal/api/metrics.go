package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the registered Prometheus collectors.
type Metrics struct {
	UploadsTotal            *prometheus.CounterVec
	UploadBytesTotal        prometheus.Counter
	PresignRequestsTotal    *prometheus.CounterVec
	VerificationDurationSec prometheus.Histogram
	WebhooksDispatchedTotal *prometheus.CounterVec
	registry                *prometheus.Registry
}

// NewMetrics initializes and registers UploadGate Prometheus metrics.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		registry: reg,
		UploadsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "uploadgate_uploads_total",
				Help: "Total count of uploads processed by final status.",
			},
			[]string{"status"},
		),
		UploadBytesTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "uploadgate_upload_bytes_total",
				Help: "Total volume of bytes verified across all uploads.",
			},
		),
		PresignRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "uploadgate_presign_requests_total",
				Help: "Total number of presign requests issued per storage provider.",
			},
			[]string{"provider"},
		),
		VerificationDurationSec: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "uploadgate_verification_duration_seconds",
				Help:    "Latency of the complete verification and thumbnailing pipeline in seconds.",
				Buckets: prometheus.DefBuckets,
			},
		),
		WebhooksDispatchedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "uploadgate_webhooks_dispatched_total",
				Help: "Total outbound webhook delivery attempts by status.",
			},
			[]string{"status"},
		),
	}

	reg.MustRegister(
		m.UploadsTotal,
		m.UploadBytesTotal,
		m.PresignRequestsTotal,
		m.VerificationDurationSec,
		m.WebhooksDispatchedTotal,
	)

	// Initialize default series so they appear immediately in Prometheus scrapes
	m.UploadsTotal.WithLabelValues("verified").Add(0)
	m.UploadsTotal.WithLabelValues("rejected").Add(0)
	m.WebhooksDispatchedTotal.WithLabelValues("success").Add(0)
	m.WebhooksDispatchedTotal.WithLabelValues("failure").Add(0)

	return m
}

// Handler returns the Prometheus HTTP metrics handler.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
