/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package events

import (
	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/touchstone/touchkit"
	"go.uber.org/fx"
)

const (
	responseCodeLabel   = "status_code"
	circuitBreakerLabel = "circuit_breaker"
)

// Measures contains the various codex client related metrics.
type Measures struct {
	fx.In
	ResponseDuration            metrics.Histogram `name:"client_response_duration"`
	CircuitBreakerStatus        metrics.Gauge     `name:"circuit_breaker_status"`
	CircuitBreakerRejectedCount metrics.Counter   `name:"circuit_breaker_rejected_count"`
	CircuitBreakerOpenDuration  metrics.Histogram `name:"circuit_breaker_open_duration"`
}

// ProvideMetrics builds the queue-related metrics and makes them available to the container.
func ProvideMetrics() fx.Option {
	return fx.Options(
		touchkit.Histogram(
			prometheus.HistogramOpts{
				Name:    "client_response_duration",
				Help:    "The amount of time it takes for codex to respond in s",
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			responseCodeLabel,
		),
		touchkit.Gauge(
			prometheus.GaugeOpts{
				Name: "circuit_breaker_status",
				Help: "The current status of the circuit breaker, with 1=open, 0.5=half-open, 0=closed",
			},
			circuitBreakerLabel,
		),
		touchkit.Counter(
			prometheus.CounterOpts{
				Name: "circuit_breaker_rejected_count",
				Help: "Number of requests rejected by the circuit breaker",
			},
			circuitBreakerLabel,
		),
		touchkit.Histogram(
			prometheus.HistogramOpts{
				Name:    "circuit_breaker_open_duration",
				Help:    "The amount of time the circuit breaker is open in s",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800},
			},
			circuitBreakerLabel,
		),
	)
}
