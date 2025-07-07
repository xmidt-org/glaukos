// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/touchstone/touchtest"
	"go.uber.org/ratelimit"
	"go.uber.org/zap"
)

func TestGetEvents(t *testing.T) {
	t.Run("build request error", testBuildRequestErr)
	t.Run("client error", testClientErr)
	t.Run("unmarshal error", testUnmarshalErr)
	t.Run("success", testSuccess)
}

func testUnmarshalErr(t *testing.T) {
	assert := assert.New(t)
	client := new(mockClient)
	auth := new(mockAcquirer)
	auth.On("Acquire").Return("test", nil)

	resp := httptest.NewRecorder()
	resp.WriteString(`{"some key": "some-value"}`)
	client.On("Do", mock.Anything).Return(resp.Result(), nil) // nolint:bodyclose
	c := CodexClient{
		Logger:         zap.NewNop(),
		Client:         client,
		CircuitBreaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test circuit breaker"}),
		Auth:           auth,
		RateLimiter:    ratelimit.NewUnlimited(),
	}
	eventsList := c.GetEvents("some-deviceID")
	assert.NotNil(eventsList)
	assert.Empty(eventsList)
}

func testClientErr(t *testing.T) {
	assert := assert.New(t)
	client := new(mockClient)
	auth := new(mockAcquirer)
	auth.On("Acquire").Return("test", nil)
	client.On("Do", mock.Anything).Return(httptest.NewRecorder().Result(), errors.New("test error")) // nolint:bodyclose
	c := CodexClient{
		Logger:         zap.NewNop(),
		Client:         client,
		CircuitBreaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test circuit breaker"}),
		Auth:           auth,
		RateLimiter:    ratelimit.NewUnlimited(),
	}
	eventsList := c.GetEvents("some-deviceID")
	assert.NotNil(eventsList)
	assert.Empty(eventsList)
}

func testBuildRequestErr(t *testing.T) {
	assert := assert.New(t)
	client := new(mockClient)
	auth := new(mockAcquirer)
	auth.On("Acquire").Return("", errors.New("auth error"))
	c := CodexClient{
		Logger:         zap.NewNop(),
		Client:         client,
		CircuitBreaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test circuit breaker"}),
		Auth:           auth,
		RateLimiter:    ratelimit.NewUnlimited(),
	}
	eventsList := c.GetEvents("some-deviceID")
	assert.NotNil(eventsList)
	assert.Empty(eventsList)
}

func testSuccess(t *testing.T) {
	assert := assert.New(t)
	client := new(mockClient)
	auth := new(mockAcquirer)
	auth.On("Acquire").Return("test", nil)

	events := []interpreter.Event{
		interpreter.Event{
			MsgType:         4,
			Source:          "source",
			Destination:     "destination",
			TransactionUUID: "112233445566",
			Birthdate:       1617152053278595600,
		},
		interpreter.Event{
			MsgType:         4,
			Destination:     "destination",
			TransactionUUID: "abcd",
			Metadata: map[string]string{
				"test-key": "test-value",
			},
			Birthdate: 1617152053278595600,
		},
	}
	resp := httptest.NewRecorder()

	jsonEvents, err := json.Marshal(events)
	assert.Nil(err)
	resp.WriteString(string(jsonEvents))
	client.On("Do", mock.Anything).Return(resp.Result(), nil) // nolint:bodyclose
	c := CodexClient{
		Logger:         zap.NewNop(),
		Client:         client,
		CircuitBreaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test circuit breaker"}),
		Auth:           auth,
		RateLimiter:    ratelimit.NewUnlimited(),
	}
	eventsList := c.GetEvents("some-deviceID")
	assert.Equal(events, eventsList)
}

func TestDoRequest(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	pastTime := true
	timeElapsed := time.Minute
	current := func() time.Time {
		if pastTime {
			pastTime = false
			return now
		}
		pastTime = true
		return now.Add(timeElapsed)
	}
	request, _ := http.NewRequest(http.MethodGet, "test-codex/test", nil)
	tests := []struct {
		description        string
		client             *mockClient
		expectedBody       []byte
		expectedStatusCode int
		clientErr          error
		expectedErr        error
	}{
		{
			description:        "success",
			client:             new(mockClient),
			expectedStatusCode: 209,
			expectedBody:       []byte("test body"),
		},
		{
			description:        "client error",
			client:             new(mockClient),
			expectedStatusCode: 500,
			clientErr:          errors.New("test"),
			expectedErr:        errors.New("test"),
		},
		{
			description:        "nil response",
			client:             new(mockClient),
			expectedStatusCode: -1,
			clientErr:          errors.New("test"),
			expectedErr:        errors.New("test"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			expectedRegistry := prometheus.NewPedanticRegistry()
			expectedHistogram := prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "testClientResponseDuration",
					Help:    "testClientResponseDuration",
					Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
				},
				[]string{responseCodeLabel},
			)
			expectedRegistry.Register(expectedHistogram)
			resp := httptest.NewRecorder()
			resp.Write(tc.expectedBody)
			resp.Code = tc.expectedStatusCode
			if tc.expectedStatusCode == -1 {
				tc.client.On("Do", request).Return(nil, tc.clientErr)
			} else {
				tc.client.On("Do", request).Return(resp.Result(), tc.clientErr) // nolint:bodyclose
			}

			m := Measures{
				ResponseDuration: prometheus.NewHistogramVec(
					prometheus.HistogramOpts{
						Name:    "testClientResponseDuration",
						Help:    "testClientResponseDuration",
						Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
					},
					[]string{responseCodeLabel},
				),
			}
			c := CodexClient{
				Client:  tc.client,
				Metrics: m,
			}
			body, err := c.doRequest(request, current)
			if tc.expectedErr != nil {
				assert.NotNil(err)
				assert.Contains(err.Error(), tc.expectedErr.Error())
			} else {
				assert.Nil(err)
				assert.NotNil(body)
				assert.Equal(string(tc.expectedBody), string(body.([]byte)))
			}
			expectedHistogram.WithLabelValues(strconv.Itoa(tc.expectedStatusCode)).Observe(timeElapsed.Seconds())
			testAssert := touchtest.New(t)
			testAssert.Expect(expectedRegistry)
			assert.True(testAssert.CollectAndCompare(m.ResponseDuration))

		})
	}
}

func TestBuildGETRequest(t *testing.T) {
	tests := []struct {
		description        string
		address            string
		auth               *mockAcquirer
		expectedAuthString string
		expectedAuthErr    error
		errExpected        error
	}{
		{
			description:        "Success",
			address:            "codex-test/test",
			auth:               new(mockAcquirer),
			expectedAuthString: "test",
		},
		{
			description:     "err with auth",
			address:         "codex-test/test",
			auth:            new(mockAcquirer),
			expectedAuthErr: errors.New("unable to create auth"),
			errExpected:     errors.New("unable to create auth"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			if tc.auth != nil {
				tc.auth.On("Acquire").Return(tc.expectedAuthString, tc.expectedAuthErr)
			}
			req, err := buildGETRequest(tc.address, tc.auth)
			if tc.errExpected == nil {
				assert.Equal(http.MethodGet, req.Method)
				assert.Equal(req.Header.Get("Authorization"), tc.expectedAuthString)
				assert.Equal(req.URL.Path, tc.address)
			} else {
				assert.NotNil(err)
				assert.Contains(err.Error(), tc.errExpected.Error())
			}
		})
	}
}

func TestExecuteRequest(t *testing.T) {
	var (
		assert = assert.New(t)
		logger = zap.NewNop()
	)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)

	tests := []struct {
		description             string
		expectedBody            []byte
		clientErr               error
		expectedCBRejectedCount float64
	}{
		{
			description:  "success",
			expectedBody: []byte("test body"),
		},
		{
			description: "client err",
			clientErr:   errors.New("test error"),
		},
		{
			description:             "circuit breaker open",
			clientErr:               gobreaker.ErrOpenState,
			expectedCBRejectedCount: 1.0,
		},
		{
			description:             "circuit breaker too many requests",
			clientErr:               gobreaker.ErrTooManyRequests,
			expectedCBRejectedCount: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client := new(mockClient)
			resp := httptest.NewRecorder()
			resp.Write(tc.expectedBody)
			client.On("Do", req).Return(resp.Result(), tc.clientErr) // nolint:bodyclose
			m := Measures{
				CircuitBreakerRejectedCount: prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "testCounter",
						Help: "testCounter",
					}, []string{circuitBreakerLabel}),
			}
			c := CodexClient{
				Logger:         logger,
				Client:         client,
				CircuitBreaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test circuit breaker"}),
				RateLimiter:    ratelimit.NewUnlimited(),
				Metrics:        m,
			}
			body, err := c.executeRequest(req)
			if tc.clientErr != nil {
				assert.Equal(tc.clientErr, err)
				assert.Nil(body)
				if tc.expectedCBRejectedCount > 0 {
					assert.Equal(tc.expectedCBRejectedCount, testutil.ToFloat64(m.CircuitBreakerRejectedCount))
				}
			} else {
				assert.Equal(string(tc.expectedBody), string(body))
				assert.Nil(err)
			}

		})
	}
}

func TestOnStateChanged(t *testing.T) {
	tests := []struct {
		description    string
		name           string
		from           gobreaker.State
		to             gobreaker.State
		expectedStatus float64
	}{
		{
			description:    "closed to open",
			name:           "test",
			from:           gobreaker.StateClosed,
			to:             gobreaker.StateOpen,
			expectedStatus: 1.0,
		},
		{
			description:    "open to half-open",
			name:           "test",
			from:           gobreaker.StateOpen,
			to:             gobreaker.StateHalfOpen,
			expectedStatus: 0.5,
		},
		{
			description:    "half-open to closed",
			name:           "test",
			from:           gobreaker.StateHalfOpen,
			to:             gobreaker.StateClosed,
			expectedStatus: 0.0,
		},
		{
			description:    "half-open to open",
			name:           "test",
			from:           gobreaker.StateHalfOpen,
			to:             gobreaker.StateOpen,
			expectedStatus: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			m := Measures{
				CircuitBreakerStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
					Name: "circuitBreakerStatus",
					Help: "circuitBreakerStatus",
				}, []string{circuitBreakerLabel}),
			}
			s := onStateChanged(m)
			s(tc.name, tc.from, tc.to)
			assert.Equal(t, tc.expectedStatus, testutil.ToFloat64(m.CircuitBreakerStatus))

		})
	}

}
