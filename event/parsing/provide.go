package parsing

import (
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sony/gobreaker"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/webpa-common/xhttp"
	"go.uber.org/fx"
)

// CodexConfig determines the auth and address for connecting to the codex cluster.
type CodexConfig struct {
	Address        string
	Auth           AuthAcquirerConfig
	MaxRetryCount  int
	CircuitBreaker CircuitBreakerConfig
}

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
		fx.Provide(
			arrange.UnmarshalKey("codex", CodexConfig{}),
			determineCodexTokenAcquirer,
			func(config CodexConfig) *gobreaker.CircuitBreaker {
				c := config.CircuitBreaker

				if c.ConsecutiveFailuresAllowed == 0 {
					c.ConsecutiveFailuresAllowed = 1
				}

				settings := gobreaker.Settings{
					Name:        "Codex Circuit Breaker",
					MaxRequests: c.MaxRequests,
					Interval:    c.Interval,
					Timeout:     c.Timeout,
					ReadyToTrip: func(count gobreaker.Counts) bool {
						if count.ConsecutiveFailures > c.ConsecutiveFailuresAllowed {
							return true
						}
						return false
					},
				}

				return gobreaker.NewCircuitBreaker(settings)
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
		level.Debug(logger).Log(xlog.MessageKey(), "using jwt")
		return acquire.NewRemoteBearerTokenAcquirer(jwt)
	}

	if config.Auth.Basic != "" {
		level.Debug(logger).Log(xlog.MessageKey(), "using basic auth")
		return acquire.NewFixedAuthAcquirer(config.Auth.Basic)
	}

	level.Error(logger).Log(xlog.ErrorKey(), "failed to create acquirer")
	return defaultAcquirer, nil

}
