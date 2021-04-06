package events

import (
	"testing"
	"time"

	"github.com/sony/gobreaker"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
)

func TestCodexTokenAcquirer(t *testing.T) {
	const (
		basicAuth  = "Authorization test"
		jwtURL     = "testURL"
		jwtBuffer  = 1 * time.Second
		jwtTimeout = 1 * time.Second

		basic = iota
		jwt
		defaultAuth
	)

	assert := assert.New(t)
	tests := []struct {
		description      string
		config           CodexConfig
		expectedAcquirer int
		expectedErr      bool
	}{
		{
			description: "Basic auth",
			config: CodexConfig{
				Address: "test",
				Auth: AuthAcquirerConfig{
					Basic: "Authorization test",
				},
			},
			expectedAcquirer: basic,
		},
		{
			description: "JWT auth",
			config: CodexConfig{
				Address: "test",
				Auth: AuthAcquirerConfig{
					JWT: acquire.RemoteBearerTokenAcquirerOptions{
						AuthURL: jwtURL,
						Timeout: jwtTimeout,
						Buffer:  jwtBuffer,
					},
				},
			},
			expectedAcquirer: jwt,
		},
		{
			description: "JWT auth-missing auth url",
			config: CodexConfig{
				Address: "test",
				Auth: AuthAcquirerConfig{
					JWT: acquire.RemoteBearerTokenAcquirerOptions{
						Timeout: jwtTimeout,
						Buffer:  jwtBuffer,
					},
				},
			},
			expectedAcquirer: defaultAuth,
		},
		{
			description: "JWT auth-missing timeout",
			config: CodexConfig{
				Address: "test",
				Auth: AuthAcquirerConfig{
					JWT: acquire.RemoteBearerTokenAcquirerOptions{
						AuthURL: jwtURL,
						Buffer:  jwtBuffer,
					},
				},
			},
			expectedAcquirer: defaultAuth,
		},
		{
			description: "JWT auth-missing buffer",
			config: CodexConfig{
				Address: "test",
				Auth: AuthAcquirerConfig{
					JWT: acquire.RemoteBearerTokenAcquirerOptions{
						AuthURL: jwtURL,
						Timeout: jwtTimeout,
					},
				},
			},
			expectedAcquirer: defaultAuth,
		},
		{
			description: "Default auth",
			config: CodexConfig{
				Address: "test",
				Auth: AuthAcquirerConfig{
					JWT: acquire.RemoteBearerTokenAcquirerOptions{},
				},
			},
			expectedAcquirer: defaultAuth,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			auth, err := determineCodexTokenAcquirer(log.NewNopLogger(), tc.config)
			if tc.expectedErr {
				assert.Nil(auth)
				assert.NotNil(err)
			} else {
				var expectedAuth acquire.Acquirer
				switch tc.expectedAcquirer {
				case basic:
					expectedAuth, _ = acquire.NewFixedAuthAcquirer(tc.config.Auth.Basic)
				case jwt:
					expectedAuth, _ = acquire.NewRemoteBearerTokenAcquirer(tc.config.Auth.JWT)
				case defaultAuth:
					expectedAuth = &acquire.DefaultAcquirer{}
				}

				assert.IsType(expectedAuth, auth)
				assert.Nil(err)
			}

		})
	}
}

func TestCreateCodexClient(t *testing.T) {
	tests := []struct {
		description string
		config      CodexConfig
	}{
		{
			description: "0 rate limit req",
			config: CodexConfig{
				RateLimit: RateLimitConfig{
					Requests: 0,
				},
			},
		},
		{
			description: "negative rate limit req",
			config: CodexConfig{
				Address: "test",
				RateLimit: RateLimitConfig{
					Requests: -1,
				},
			},
		},
		{
			description: "0 per for rate limiting",
			config: CodexConfig{
				Address: "random",
				RateLimit: RateLimitConfig{
					Requests: 10,
					Tick:     0,
				},
			},
		},
		{
			description: "negative per for rate limiting",
			config: CodexConfig{
				Address: "test",
				RateLimit: RateLimitConfig{
					Requests: 10,
					Tick:     -1 * time.Second,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			p := xmetricstest.NewProvider(&xmetrics.Options{})
			m := Measures{
				CircuitBreakerStatus: p.NewGauge("circuit_breaker_status"),
			}
			auth := &acquire.DefaultAcquirer{}
			logger := log.NewNopLogger()
			cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test"})
			client := createCodexClient(tc.config, cb, auth, m, logger)
			assert.NotNil(client)
			assert.Equal(tc.config.Address, client.Address)
			assert.Equal(auth, client.Auth)
			assert.Equal(m, client.Metrics)
			assert.Equal(cb, client.CircuitBreaker)
			p.Assert(t, "circuit_breaker_status", circuitBreakerLabel, cb.Name())(xmetricstest.Value(0.0))
		})
	}
}
