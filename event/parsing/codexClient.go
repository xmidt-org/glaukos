package parsing

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/sony/gobreaker"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/webpa-common/logging"
	"github.com/xmidt-org/webpa-common/xhttp"
)

type CodexClient struct {
	Address string
	Auth    acquire.Acquirer

	retryOptions xhttp.RetryOptions
	client       *http.Client
	cb           *gobreaker.CircuitBreaker
	logger       log.Logger
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
		logging.Error(c.logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to build request")
		return eventList
	}

	status, data, err := c.doRequest(request)
	if err != nil {
		logging.Error(c.logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to complete request")
		return eventList
	}

	if status != 200 {
		logging.Error(c.logger).Log("status", status, logging.MessageKey(), "non 200", "url", request.URL)
		return eventList
	}

	if err = json.Unmarshal(data, &eventList); err != nil {
		logging.Error(c.logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to read body")
		return eventList
	}

	return eventList
}

func (c *CodexClient) doRequest(request *http.Request) (int, []byte, error) {
	f := func(req *http.Request) (*http.Response, error) {
		body, err := c.cb.Execute(func() (interface{}, error) {
			fmt.Println("inside woot")
			return c.client.Do(req)
		})

		if err != nil {
			return nil, err
		}

		b, ok := body.(*http.Response)

		if !ok {
			return nil, errors.New("failed to convert body to http response")
		}

		return b, nil

	}
	response, err := xhttp.RetryTransactor(c.retryOptions, f)(request)
	if err != nil {
		logging.Error(c.logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "RetryTransactor failed")
		return 0, []byte{}, err
	}

	data, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		logging.Error(c.logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to read body")
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
