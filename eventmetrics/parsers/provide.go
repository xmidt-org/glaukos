package parsers

import (
	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/glaukos/eventmetrics/queue"
	"github.com/xmidt-org/glaukos/message/history"
	"go.uber.org/fx"
)

// Provide bundles everything needed for setting up all of the event objects
// for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideEventMetrics(),
		history.Provide(),
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
		fx.Annotated{
			Group: "parsers",
			Target: func(logger log.Logger, measures Measures, client *history.CodexClient) queue.Parser {
				return &BootTimeParser{
					Measures: measures,
					Logger:   logger,
					Client:   client,
				}
			},
		},
		fx.Annotated{
			Group: "parsers",
			Target: func(logger log.Logger, measures Measures, client *history.CodexClient) queue.Parser {
				return &RebootTimeParser{
					Measures: measures,
					Logger:   logger,
					Client:   client,
					Label:    "reboot_to_manageable_parser",
				}
			},
		},
	)
}
