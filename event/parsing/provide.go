package parsing

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/sony/gobreaker"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/webpa-common/logging"
	"github.com/xmidt-org/webpa-common/xhttp"
	"go.uber.org/fx"
)

// CodexConfig determines the auth and address for connecting to the codex cluster.
type CodexConfig struct {
	Address       string
	Auth          AuthAcquirerConfig
	MaxRetryCount int
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
		fx.Provide(
			arrange.UnmarshalKey("codex", CodexConfig{}),
			determineCodexTokenAcquirer,
			func() *gobreaker.CircuitBreaker {
				s := gobreaker.Settings{
					Name:        "Codex Circuit Breaker",
					MaxRequests: 0,
					Interval:    1 * time.Minute,
					Timeout:     2 * time.Minute,
					ReadyToTrip: func(count gobreaker.Counts) bool {
						if count.ConsecutiveFailures > 10 {
							return true
						}

						return false
					},
				}

				return gobreaker.NewCircuitBreaker(s)
			},
			func(config CodexConfig, cb *gobreaker.CircuitBreaker, codexAuth acquire.Acquirer, logger log.Logger) EventClient {
				if config.MaxRetryCount < 0 {
					config.MaxRetryCount = 3
				}
				retryOptions := xhttp.RetryOptions{
					Logger:   logger,
					Retries:  config.MaxRetryCount,
					Interval: time.Second * 30,

					// Always retry on failures up to the max count.
					ShouldRetry:       func(error) bool { return true },
					ShouldRetryStatus: func(code int) bool { return false },
				}

				fmt.Println(retryOptions.Retries)

				return &CodexClient{
					Address:      config.Address,
					Auth:         codexAuth,
					retryOptions: retryOptions,
					client:       http.DefaultClient,
					logger:       logger,
					cb:           cb,
				}
			},
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
				Target: func(logger log.Logger, measures Measures, client EventClient) queue.Parser {
					return BootTimeParser{
						Measures: measures,
						Logger:   logger,
						Client:   client,
					}
				},
			},
			func(parsers ParsersIn) []queue.Parser {
				return parsers.Parsers
			},
		),
	)
}

func determineCodexTokenAcquirer(logger log.Logger, config CodexConfig) (acquire.Acquirer, error) {
	defaultAcquirer := &acquire.DefaultAcquirer{}
	jwt := config.Auth.JWT
	if jwt.AuthURL != "" && jwt.Buffer > 0 && jwt.Timeout > 0 {
		logging.Debug(logger).Log(logging.MessageKey(), "using jwt")
		return acquire.NewRemoteBearerTokenAcquirer(jwt)
	}

	if config.Auth.Basic != "" {
		logging.Debug(logger).Log(logging.MessageKey(), "using basic")
		return acquire.NewFixedAuthAcquirer(config.Auth.Basic)
	}

	logging.Error(logger).Log(logging.MessageKey(), "failed to create acquirer")
	return defaultAcquirer, nil

}
