package events

import (
	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/themis/xmetrics"
	"go.uber.org/fx"
)

const (
	responseCodeLabel   = "status_code"
	circuitBreakerLabel = "circuit_breaker"
)

// Measures contains the various codex client related metrics.
type Measures struct {
	fx.In
	RequestCount                metrics.Counter   `name:"client_request_count"`
	ResponseCount               metrics.Counter   `name:"client_response_count"`
	CircuitBreakerOpenCount     metrics.Counter   `name:"circuit_breaker_open_count"`
	CircuitBreakerRejectedCount metrics.Counter   `name:"circuit_breaker_rejected_count"`
	CircuitBreakerOpenDuration  metrics.Histogram `name:"circuit_breaker_open_duration"`
}

// ProvideMetrics builds the queue-related metrics and makes them available to the container.
func ProvideMetrics() fx.Option {
	return fx.Provide(
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "client_request_count",
				Help: "Number of requests attempted",
			},
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "client_response_count",
				Help: "Number of responses, broken down by response code",
			},
			responseCodeLabel,
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "circuit_breaker_open_count",
				Help: "Number of times the circuit breaker was activated",
			},
			circuitBreakerLabel,
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "circuit_breaker_rejected_count",
				Help: "Number of requests rejected by the circuit breaker",
			},
			circuitBreakerLabel,
		),
		xmetrics.ProvideHistogram(
			prometheus.HistogramOpts{
				Name:    "circuit_breaker_open_duration",
				Help:    "The amount of time the circuit breaker is open",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800},
			},
			circuitBreakerLabel,
		),
	)
}
