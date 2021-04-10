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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
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
		return c.doRequest(request, time.Now)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			c.Metrics.CircuitBreakerRejectedCount.With(prometheus.Labels{circuitBreakerLabel: c.CircuitBreaker.Name()}).Add(1.0)
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

func (c *CodexClient) doRequest(req *http.Request, currentTime func() time.Time) (interface{}, error) {
	requestBegin := currentTime()
	resp, err := c.Client.Do(req)
	timeElapsed := currentTime().Sub(requestBegin).Seconds()

	if resp != nil && c.Metrics.ResponseDuration != nil {
		c.Metrics.ResponseDuration.With(prometheus.Labels{responseCodeLabel: strconv.Itoa(resp.StatusCode)}).Observe(timeElapsed)
	} else if resp == nil && c.Metrics.ResponseDuration != nil {
		c.Metrics.ResponseDuration.With(prometheus.Labels{responseCodeLabel: "-1"}).Observe(timeElapsed)
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
		if m.CircuitBreakerStatus != nil {
			switch to {
			case gobreaker.StateClosed:
				m.CircuitBreakerStatus.With(prometheus.Labels{circuitBreakerLabel: name}).Set(0.0)
			case gobreaker.StateHalfOpen:
				m.CircuitBreakerStatus.With(prometheus.Labels{circuitBreakerLabel: name}).Set(0.5)
			case gobreaker.StateOpen:
				m.CircuitBreakerStatus.With(prometheus.Labels{circuitBreakerLabel: name}).Set(1.0)
			}
		}

		if from == gobreaker.StateClosed && to == gobreaker.StateOpen {
			start = time.Now()
		} else if to == gobreaker.StateClosed && m.CircuitBreakerOpenDuration != nil {
			openTime := time.Since(start).Seconds()
			m.CircuitBreakerOpenDuration.With(prometheus.Labels{circuitBreakerLabel: name}).Observe(openTime)
		}
	}
}
