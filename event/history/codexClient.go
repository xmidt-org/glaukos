package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/sony/gobreaker"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/webpa-common/xhttp"
)

type CircuitBreakerConfig struct {
	MaxRequests                uint32
	Interval                   time.Duration
	Timeout                    time.Duration
	ConsecutiveFailuresAllowed uint32
}

type CodexClient struct {
	Address      string
	Auth         acquire.Acquirer
	RetryOptions xhttp.RetryOptions
	Client       *http.Client
	CB           *gobreaker.CircuitBreaker
	Logger       log.Logger
}

// Event is the struct that codex query results will be unmarshalled into.
type Event struct {
	MsgType         int               `json:"msg_type"`
	Source          string            `json:"source"`
	Dest            string            `json:"dest,omitempty"`
	TransactionUUID string            `json:"transaction_uuid,omitempty"`
	ContentType     string            `json:"content_type,omitempty"`
	Metadata        map[string]string `json:"metadata"`
	Payload         string            `json:"payload,omitempty"`
	BirthDate       int64             `json:"birth_date"`
	PartnerIDs      []string          `json:"partner_ids,omitempty"`
}

// GetEvents queries codex for events related to a device.
func (c *CodexClient) GetEvents(device string) []Event {
	eventList := make([]Event, 0)

	request, err := buildGetRequest(fmt.Sprintf("%s/api/v1/device/%s/events", c.Address, device), c.Auth)
	if err != nil {
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to build request")
		return eventList
	}

	status, data, err := c.doRequest(request)
	if err != nil {
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to complete request")
		return eventList
	}

	if status != 200 {
		level.Error(c.Logger).Log("status", status, xlog.MessageKey(), "non 200", "url", request.URL)
		return eventList
	}

	if err = json.Unmarshal(data, &eventList); err != nil {
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to read body")
		return eventList
	}

	return eventList
}

func (c *CodexClient) doRequest(request *http.Request) (int, []byte, error) {
	f := circuitBreakerRequestFunc(c)
	response, err := xhttp.RetryTransactor(c.RetryOptions, f)(request)
	if err != nil {
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "RetryTransactor failed")
		return 0, []byte{}, err
	}

	data, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		level.Error(c.Logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to read body")
		return 0, []byte{}, err
	}

	return response.StatusCode, data, nil
}

func buildGetRequest(address string, auth acquire.Acquirer) (*http.Request, error) {
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}

	if err := acquire.AddAuth(request, auth); err != nil {
		return nil, err
	}

	return request, nil
}

func circuitBreakerRequestFunc(c *CodexClient) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		resp, err := c.CB.Execute(func() (interface{}, error) {
			return c.Client.Do(req)
		})

		if err != nil {
			return nil, err
		}

		if b, ok := resp.(*http.Response); !ok {
			return nil, errors.New("failed to convert response to a http response")
		} else {
			return b, nil
		}

	}
}
