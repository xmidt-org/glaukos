package parsing

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/webpa-common/logging"
	"github.com/xmidt-org/webpa-common/xhttp"
)

type CodexClient struct {
	Address string
	Auth    acquire.Acquirer

	logger log.Logger
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

// query codex for events related to a device
func (c *CodexClient) GetEvents(device string) []Event {
	eventList := make([]Event, 0)

	request, err := buildRequest(fmt.Sprintf("%s/api/v1/device/%s/events", c.Address, device), c.Auth)
	if err != nil {
		logging.Error(c.logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to build request")
		return eventList
	}

	status, data, err := doRequest(request, c.logger)
	if err != nil {
		logging.Error(c.logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to complete request")
		return eventList
	}

	if status != 200 {
		logging.Error(c.logger).Log("status", status, logging.MessageKey(), "non 200", "url", request.URL)
		return eventList
	}

	err = json.Unmarshal(data, &eventList)
	if err != nil {
		logging.Error(c.logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to read body")
		return eventList
	}

	return eventList
}

func buildRequest(address string, auth acquire.Acquirer) (*http.Request, error) {
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}

	if err := acquire.AddAuth(request, auth); err != nil {
		return nil, err
	}

	return request, nil
}

func doRequest(request *http.Request, logger log.Logger) (int, []byte, error) {
	retryOptions := xhttp.RetryOptions{
		Logger:   logger,
		Retries:  3,
		Interval: time.Second * 30,

		// Always retry on failures up to the max count.
		ShouldRetry:       func(error) bool { return true },
		ShouldRetryStatus: func(code int) bool { return false },
	}

	response, err := xhttp.RetryTransactor(retryOptions, http.DefaultClient.Do)(request)
	if err != nil {
		logging.Error(logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "RetryTransactor failed")
		return 0, []byte{}, err
	}

	data, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		logging.Error(logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to read body")
		return 0, []byte{}, err
	}

	return response.StatusCode, data, nil
}
