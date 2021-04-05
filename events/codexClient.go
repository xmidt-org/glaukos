package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sony/gobreaker"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/httpaux"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/themis/xlog"
	"go.uber.org/ratelimit"
)

// CodexClient is the client used to get events from codex.
type CodexClient struct {
	Address        string
	Auth           acquire.Acquirer
	Client         httpaux.Client
	CircuitBreaker *gobreaker.CircuitBreaker
	RateLimiter    ratelimit.Limiter
	Logger         log.Logger
	Metrics        Measures
}

// GetEvents queries codex for events related to a device.
func (c *CodexClient) GetEvents(device string) []interpreter.Event {
	eventList := make([]interpreter.Event, 0)

	request, err := buildGETRequest(fmt.Sprintf("%s/api/v1/device/%s/events", c.Address, device), c.Auth)
	if err != nil {
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to build request")
		return eventList
	}

	data, err := c.executeRequest(request)
	if err != nil {
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to complete request")
		return eventList
	}

	if err = json.Unmarshal(data, &eventList); err != nil {
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to read body")
		return eventList
	}

	return eventList
}

func (c *CodexClient) executeRequest(request *http.Request) ([]byte, error) {
	c.RateLimiter.Take()
	response, err := c.CircuitBreaker.Execute(func() (interface{}, error) {
		return c.doRequest(request)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			c.Metrics.CircuitBreakerRejectedCount.With(circuitBreakerLabel, c.CircuitBreaker.Name()).Add(1.0)
		}
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to make request")
		return nil, err
	}

	r, ok := response.([]byte)
	if !ok {
		return nil, errors.New("failed to convert body to byte array")
	}

	return r, nil
}

func (c *CodexClient) doRequest(req *http.Request) (interface{}, error) {
	if c.Metrics.RequestCount != nil {
		c.Metrics.RequestCount.Add(1.0)
	}

	resp, err := c.Client.Do(req)
	if resp != nil && c.Metrics.ResponseCount != nil {
		c.Metrics.ResponseCount.With(responseCodeLabel, strconv.Itoa(resp.StatusCode)).Add(1.0)
	}

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %w", err)
	}
	return body, nil
}

func buildGETRequest(address string, auth acquire.Acquirer) (*http.Request, error) {
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}

	if err := acquire.AddAuth(request, auth); err != nil {
		return nil, err
	}

	return request, nil
}

// logs prometheus metrics when circuit breaker state changes
func onStateChanged(m Measures) func(string, gobreaker.State, gobreaker.State) {
	var start time.Time
	return func(name string, from gobreaker.State, to gobreaker.State) {
		if from == gobreaker.StateClosed && to == gobreaker.StateOpen && m.CircuitBreakerOpenCount != nil {
			m.CircuitBreakerOpenCount.With(circuitBreakerLabel, name).Add(1.0)
			start = time.Now()
		} else if to == gobreaker.StateClosed && m.CircuitBreakerOpenDuration != nil {
			openTime := time.Since(start).Seconds()
			m.CircuitBreakerOpenDuration.With(circuitBreakerLabel, name).Observe(openTime)
		}
	}
}
