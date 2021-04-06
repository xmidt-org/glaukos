package events

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
	"go.uber.org/ratelimit"
)

func TestGetEvents(t *testing.T) {
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
		Logger:         log.NewNopLogger(),
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
		Logger:         log.NewNopLogger(),
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
		Logger:         log.NewNopLogger(),
		Client:         client,
		CircuitBreaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test circuit breaker"}),
		Auth:           auth,
		RateLimiter:    ratelimit.NewUnlimited(),
	}
	eventsList := c.GetEvents("some-deviceID")
	assert.Equal(events, eventsList)
}

func TestDoRequest(t *testing.T) {
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
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			resp := httptest.NewRecorder()
			resp.Write(tc.expectedBody)
			resp.Code = tc.expectedStatusCode
			tc.client.On("Do", request).Return(resp.Result(), tc.clientErr) // nolint:bodyclose
			c := CodexClient{
				Client: tc.client,
			}
			body, err := c.doRequest(request)
			if tc.expectedErr != nil {
				assert.NotNil(err)
				assert.Contains(err.Error(), tc.expectedErr.Error())
			} else {
				assert.Nil(err)
				assert.NotNil(body)
				assert.Equal(string(tc.expectedBody), string(body.([]byte)))
			}

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
		logger = log.NewNopLogger()
	)

	req, _ := http.NewRequest(http.MethodGet, "/", nil)

	tests := []struct {
		description  string
		expectedBody []byte
		clientErr    error
	}{
		{
			description:  "success",
			expectedBody: []byte("test body"),
		},
		{
			description: "client err",
			clientErr:   errors.New("test error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client := new(mockClient)
			resp := httptest.NewRecorder()
			resp.Write(tc.expectedBody)
			client.On("Do", req).Return(resp.Result(), tc.clientErr) // nolint:bodyclose
			c := CodexClient{
				Logger:         logger,
				Client:         client,
				CircuitBreaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{Name: "test circuit breaker"}),
				RateLimiter:    ratelimit.NewUnlimited(),
			}
			body, err := c.executeRequest(req)
			if tc.clientErr != nil {
				assert.Equal(tc.clientErr, err)
				assert.Nil(body)
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
			p := xmetricstest.NewProvider(&xmetrics.Options{})
			m := Measures{
				CircuitBreakerStatus: p.NewGauge("circuit_breaker_status"),
			}
			s := onStateChanged(m)
			s(tc.name, tc.from, tc.to)
			p.Assert(t, "circuit_breaker_status", circuitBreakerLabel, tc.name)(xmetricstest.Value(tc.expectedStatus))

		})
	}

}
