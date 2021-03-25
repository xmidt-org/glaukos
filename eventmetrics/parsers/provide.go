package parsers

import (
	"github.com/xmidt-org/glaukos/eventmetrics/queue"
	"github.com/xmidt-org/glaukos/events"
	"go.uber.org/fx"
)

// Provide bundles everything needed for setting up all of the event objects
// for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideEventMetrics(),
		events.Provide(),
		provideParsers(),
	)
}

func provideParsers() fx.Option {
	return fx.Provide(
		fx.Annotated{
			Group: "parsers",
			Target: func(measures Measures) queue.Parser {
				return MetadataParser{
					Measures: measures,
				}
			},
		},
	)
}
