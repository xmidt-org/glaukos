/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package eventqueue

import (
	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/themis/xmetrics"
	"go.uber.org/fx"
)

const (
	partnerIDLabel = "partner_id"
)

// QueueMetricsIn contains the various queue-related metrics
type QueueMetricsIn struct {
	fx.In
	EventQueue  metrics.Gauge   `name:"event_queue_depth"`
	EventsCount metrics.Counter `name:"event_count"`
}

// ProvideQueueMetrics builds the queue-related metrics and makes them available to the container.
func ProvideQueueMetrics() fx.Option {
	return fx.Provide(
		xmetrics.ProvideGauge(
			prometheus.GaugeOpts{
				Name: "event_queue_depth",
				Help: "The depth of the event queue",
			},
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "event_count",
				Help: "Details of incoming events",
			},
			partnerIDLabel,
		),
	)
}
