package parsing

import (
	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/themis/xmetrics"
	"go.uber.org/fx"
)

const (
	FirmwareLabel = "firmware"
	HardwareLabel = "hardware"
	ParserLabel   = "parser_type"
	ReasonLabel   = "reason"
)

// MetricsIn tracks the various event-related metrics.
type MetricsIn struct {
	fx.In
	MetadataFields        metrics.Counter   `name:"metadata_fields"`
	BootTimeHistogram     metrics.Histogram `name:"boot_time_duration"`
	UnparsableEventsCount metrics.Counter   `name:"unparsable_events_count"`
}

// ProvideEventMetrics builds the event-related metrics and makes them available to the container.
func ProvideEventMetrics() fx.Option {
	return fx.Provide(
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "metadata_fields",
				Help: "the metadata fields coming from each event received",
			},
			MetadataKeyLabel,
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "unparsable_events_count",
				Help: "events that are unparsable, labelled by type of parser and the reason why they failed",
			},
			ParserLabel,
			ReasonLabel,
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
