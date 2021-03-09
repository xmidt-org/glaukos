package client

import (
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sony/gobreaker"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/bascule/acquire"
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

type AuthAcquirerConfig struct {
	JWT   acquire.RemoteBearerTokenAcquirerOptions
	Basic string
}

// Provide bundles everything needed for setting up all of the event objects
// for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Provide(
		arrange.UnmarshalKey("codex", CodexConfig{}),
		determineCodexTokenAcquirer,
		createCircuitBreaker,
		func(config CodexConfig, cb *gobreaker.CircuitBreaker, codexAuth acquire.Acquirer, logger log.Logger) *CodexClient {
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
				RetryOptions: retryOptions,
				Client:       http.DefaultClient,
				Logger:       logger,
				CB:           cb,
			}
		},
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

func createCircuitBreaker(config CodexConfig) *gobreaker.CircuitBreaker {
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
			return count.ConsecutiveFailures >= c.ConsecutiveFailuresAllowed
		},
	}

	return gobreaker.NewCircuitBreaker(settings)
}
