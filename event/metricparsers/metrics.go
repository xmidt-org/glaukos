package metricparsers

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

// Measures tracks the various event-related metrics.
type Measures struct {
	fx.In
	UnparsableEventsCount metrics.Counter              `name:"unparsable_events_count"`
	MetadataFields        metrics.Counter              `name:"metadata_fields"`
	TimeElapsedHistograms map[string]metrics.Histogram `name:"time_elapsed_histograms"`
}

func (m *Measures) addTimeElapsedHistogram(f xmetrics.Factory, o prometheus.HistogramOpts, labelNames ...string) (bool, error) {
	histogram, err := f.NewHistogram(o, labelNames)
	if err != nil {
		return false, err
	}
	m.TimeElapsedHistograms[o.Name] = histogram
	return true, nil
}

// ProvideEventMetrics builds the event-related metrics and makes them available to the container.
func ProvideEventMetrics() fx.Option {
	return fx.Provide(
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "unparsable_events_count",
				Help: "events that are unparsable, labelled by type of parser and the reason why they failed",
			},
			ParserLabel,
			ReasonLabel,
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "metadata_fields",
				Help: "the metadata fields coming from each event received",
			},
			MetadataKeyLabel,
		),
		fx.Annotated{
			Name:   "time_elapsed_histograms",
			Target: func() map[string]metrics.Histogram { return make(map[string]metrics.Histogram) },
		},
	)
}
