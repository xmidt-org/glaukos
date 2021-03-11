package metricparsers

import (
	"time"

	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/queue"
	"go.uber.org/fx"
)

type CircuitBreakerConfig struct {
	MaxRequests                uint32
	Interval                   time.Duration
	Timeout                    time.Duration
	ConsecutiveFailuresAllowed uint32
}

type AuthAcquirerConfig struct {
	JWT   acquire.RemoteBearerTokenAcquirerOptions
	Basic string
}

// ParsersIn brings together all of the different types of parsers that glaukos uses.
type ParsersIn struct {
	fx.In
	Parsers []queue.Parser `group:"parsers"`
}

// Provide bundles everything needed for setting up all of the event objects
// for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideEventMetrics(),
		client.Provide(),
		provideParsers(),
		fx.Provide(
			func(parsers ParsersIn) []queue.Parser {
				return parsers.Parsers
			},
		),
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
			Target: func(logger log.Logger, measures Measures, client *client.CodexClient) queue.Parser {
				return &BootTimeParser{
					Measures: measures,
					Logger:   logger,
					Client:   client,
				}
			},
		},
		fx.Annotated{
			Group: "parsers",
			Target: func(logger log.Logger, measures Measures, client *client.CodexClient) queue.Parser {
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
