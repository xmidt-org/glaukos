package client

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/bascule/acquire"
)

func TestBuildGetRequest(t *testing.T) {
	assert := assert.New(t)
	fixedAuth, _ := acquire.NewFixedAuthAcquirer("test")
	tests := []struct {
		description string
		address     string
		auth        acquire.Acquirer
		errExpected bool
	}{
		{
			description: "Success",
			address:     "http://foo.com/test",
			auth:        fixedAuth,
		},
		{
			description: "Nil Auth",
			address:     "http://foo.com/test",
			auth:        nil,
			errExpected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			req, err := buildGetRequest(tc.address, tc.auth)

			if tc.errExpected {
				assert.Nil(req)
				assert.NotNil(err)
			} else {
				assert.Equal(http.MethodGet, req.Method)
				assert.NotEmpty(req.Header.Get("Authorization"))
				assert.Nil(err)
			}

		})
	}
}

func TestDoRequest(t *testing.T) {
	var (
		assert = assert.New(t)
		c      = CodexClient{
			Logger: log.NewNopLogger(),
			Client: http.DefaultClient,
			CB:     gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test circuit breaker"}),
		}
	)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)

	code, data, err := c.doRequest(req)
	assert.Equal(0, code)
	assert.Equal(0, len(data))
	assert.NotNil(err)
}

func TestGetEvents(t *testing.T) {
	assert := assert.New(t)
	auth, _ := acquire.NewFixedAuthAcquirer("test")

	tests := []struct {
		description string
		address     string
		auth        acquire.Acquirer
		errExpected bool
	}{
		{
			description: "Problem building request",
			address:     "test",
			auth:        nil,
		},
		{
			description: "Invalid URL",
			address:     "test",
			auth:        auth,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			c := CodexClient{
				Logger:  log.NewNopLogger(),
				Client:  http.DefaultClient,
				Address: tc.address,
				Auth:    auth,
				CB:      gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test circuit breaker"}),
			}

			events := c.GetEvents("test-device")
			assert.Empty(events)
		})
	}

}

func TestCircuitBreakerRequestFunc(t *testing.T) {
	const (
		failuresAllowed = 3
	)

	settings := gobreaker.Settings{
		Name:        "Codex Circuit Breaker",
		MaxRequests: 0,
		Interval:    0,
		ReadyToTrip: func(count gobreaker.Counts) bool {
			return count.ConsecutiveFailures >= failuresAllowed
		},
	}
	testCodexClient := &CodexClient{
		CB:     gobreaker.NewCircuitBreaker(settings),
		Client: &http.Client{},
	}

	f := circuitBreakerRequestFunc(testCodexClient)
	req, _ := http.NewRequest(http.MethodGet, "foo.com/test", nil)

	for i := 0; i < failuresAllowed; i++ {
		f(req)
	}

	resp, err := f(req)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, gobreaker.ErrOpenState))
}
