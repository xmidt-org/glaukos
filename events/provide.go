/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package events

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/httpaux/retry"
	"go.uber.org/fx"
	"go.uber.org/ratelimit"
	"go.uber.org/zap"
)

// CodexConfig determines the auth and address for connecting to the codex cluster.
type CodexConfig struct {
	Address        string
	Auth           AuthAcquirerConfig
	MaxRetryCount  int
	RateLimit      RateLimitConfig
	CircuitBreaker CircuitBreakerConfig
}

// CircuitBreakerConfig deals with configuration for the circuit breaker.
type CircuitBreakerConfig struct {
	MaxRequests                uint32
	Interval                   time.Duration
	Timeout                    time.Duration
	ConsecutiveFailuresAllowed uint32
}

// RateLimitConfig is the configuration for the rate limiter.
type RateLimitConfig struct {
	Requests int
	Tick     time.Duration
}

// AuthAcquirerConfig is the auth config for the client making requests to get a device's history of events.
type AuthAcquirerConfig struct {
	JWT   acquire.RemoteBearerTokenAcquirerOptions
	Basic string
}

// Provide bundles everything needed for setting up all of the event objects
// for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideMetrics(),
		fx.Provide(
			arrange.UnmarshalKey("codex", CodexConfig{}),
			determineCodexTokenAcquirer,
			createCircuitBreaker,
			onStateChanged,
			createCodexClient,
		),
	)

}

func createCodexClient(config CodexConfig, cb *gobreaker.CircuitBreaker, codexAuth acquire.Acquirer, measures Measures, logger *zap.Logger) *CodexClient {
	var limiter ratelimit.Limiter
	if config.RateLimit.Requests <= 0 {
		limiter = ratelimit.NewUnlimited()
	} else {
		if config.RateLimit.Tick <= 0 {
			config.RateLimit.Tick = time.Second
		}

		limiter = ratelimit.New(config.RateLimit.Requests, ratelimit.Per(config.RateLimit.Tick), ratelimit.WithoutSlack)
	}
	retryConfig := retry.Config{
		Retries:  config.MaxRetryCount,
		Interval: time.Second * 30,
	}

	client := retry.New(retryConfig, new(http.Client))

	if measures.CircuitBreakerStatus != nil {
		measures.CircuitBreakerStatus.With(prometheus.Labels{circuitBreakerLabel: cb.Name()}).Set(0.0)
	}

	return &CodexClient{
		Address:        config.Address,
		Auth:           codexAuth,
		Client:         client,
		Logger:         logger,
		RateLimiter:    limiter,
		Metrics:        measures,
		CircuitBreaker: cb,
	}
}

func determineCodexTokenAcquirer(logger *zap.Logger, config CodexConfig) (acquire.Acquirer, error) {
	defaultAcquirer := &acquire.DefaultAcquirer{}
	jwt := config.Auth.JWT
	if jwt.AuthURL != "" && jwt.Buffer > 0 && jwt.Timeout > 0 {
		logger.Debug("using jwt")
		return acquire.NewRemoteBearerTokenAcquirer(jwt)
	}

	if config.Auth.Basic != "" {
		logger.Debug("using basic auth")
		return acquire.NewFixedAuthAcquirer(config.Auth.Basic)
	}

	logger.Error("failed to create acquirer")
	return defaultAcquirer, nil

}

func createCircuitBreaker(config CodexConfig, onStateChange func(string, gobreaker.State, gobreaker.State)) *gobreaker.CircuitBreaker {
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
		OnStateChange: onStateChange,
	}

	return gobreaker.NewCircuitBreaker(settings)
}
