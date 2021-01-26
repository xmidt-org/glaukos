/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package queue

import (
	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/themis/xmetrics"
	"go.uber.org/fx"
)

const (
	partnerIDLabel  = "partner_id"
	reasonLabel     = "reason"
	queueFullReason = "queue_full"
)

// Measures contains the various queue-related metrics.
type Measures struct {
	fx.In
	EventsQueueDepth   metrics.Gauge   `name:"events_queue_depth"`
	EventsCount        metrics.Counter `name:"events_count"`
	DroppedEventsCount metrics.Counter `name:"dropped_events_count"`
}

// ProvideMetrics builds the queue-related metrics and makes them available to the container.
func ProvideMetrics() fx.Option {
	return fx.Provide(
		xmetrics.ProvideGauge(
			prometheus.GaugeOpts{
				Name: "events_queue_depth",
				Help: "The depth of the event queue",
			},
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "events_count",
				Help: "Details of incoming events",
			},
			partnerIDLabel,
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "dropped_events_count",
				Help: "The total number of events dropped",
			},
			reasonLabel,
		),
	)
}
