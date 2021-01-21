/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package event

import (
	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/themis/xmetrics"
	"go.uber.org/fx"
)

const (
	KeyLabel       = "metadata_key"
	FirmwareLabel  = "firmware"
	HardwareLabel  = "hardware"
	partnerIDLabel = "partner_id"
)

// MetricsIn tracks the various event-related metrics
type MetricsIn struct {
	fx.In
	MetadataFields    metrics.Counter   `name:"metadata_fields"`
	BootTimeHistogram metrics.Histogram `name:"boot_time_duration"`
}

// ProvideEventMetrics builds the event-related metrics and makes them available to the container.
func ProvideEventMetrics() fx.Option {
	return fx.Provide(
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "metadata_fields",
				Help: "the metadata fields coming from each event received",
			},
			KeyLabel,
		),
		xmetrics.ProvideHistogram(
			prometheus.HistogramOpts{
				Name:    "boot_time_duration",
				Help:    "tracks boot time durations in s",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600},
			},
			FirmwareLabel,
			HardwareLabel,
		),
	)
}

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
