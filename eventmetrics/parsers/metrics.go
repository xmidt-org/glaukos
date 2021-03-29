package parsers

import (
	"errors"
	"fmt"

	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/themis/xmetrics"
	"go.uber.org/fx"
)

const (
	ParserLabel = "parser_type"
	ReasonLabel = "reason"
)

var (
	errNilFactory   = errors.New("factory cannot be nil")
	errNewHistogram = errors.New("unable to create new histogram")
)

// Measures tracks the various event-related metrics.
type Measures struct {
	fx.In
	MetadataFields        metrics.Counter              `name:"metadata_fields"`
	TimeElapsedHistograms map[string]metrics.Histogram `name:"time_elapsed_histograms"`
	UnparsableEventsCount metrics.Counter              `name:"unparsable_events_count"`
}

func (m *Measures) addTimeElapsedHistogram(f xmetrics.Factory, o prometheus.HistogramOpts, labelNames ...string) (bool, error) {
	if f == nil {
		return false, errNilFactory
	}

	histogram, err := f.NewHistogram(o, labelNames)
	if err != nil {
		return false, fmt.Errorf("%w: %v", errNewHistogram, err)
	}

	if m.TimeElapsedHistograms == nil {
		m.TimeElapsedHistograms = make(map[string]metrics.Histogram)
	}

	if _, found := m.TimeElapsedHistograms[o.Name]; found {
		return false, fmt.Errorf("%w: histogram already exists", errNewHistogram)
	}

	m.TimeElapsedHistograms[o.Name] = histogram
	return true, nil
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
		fx.Annotated{
			Name:   "time_elapsed_histograms",
			Target: func() map[string]metrics.Histogram { return make(map[string]metrics.Histogram) },
		},
	)
}
