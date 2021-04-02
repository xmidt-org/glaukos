package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

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
		return doRequest(c.Client, request)
	})

	if err != nil {
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to make request")
		return nil, err
	}

	r, ok := response.([]byte)
	if !ok {
		return nil, errors.New("failed to convert body to byte array")
	}

	return r, nil
}

func doRequest(client httpaux.Client, req *http.Request) (interface{}, error) {
	resp, err := client.Do(req)
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
