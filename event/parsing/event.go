/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package parsing

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/webpa-common/logging"
	"github.com/xmidt-org/webpa-common/xhttp"
	"github.com/xmidt-org/wrp-go/v3"
)

const (
	hardwareKey = "/hw-model"
	firmwareKey = "/fw-name"
	bootTimeKey = "/boot-time"
)

type Parser interface {
	Parse(wrp.Message) error
}

type MetadataParser struct {
	MetadataFields metrics.Counter `name:"metadata_fields"`
}

func (m MetadataParser) Parse(msg wrp.Message) error {
	if len(msg.Metadata) < 1 {
		return errors.New("no metadata found")
	}
	for key := range msg.Metadata {
		m.MetadataFields.With(KeyLabel, key).Add(1.0)
	}
	return nil
}

type BootTimeCalc struct {
	BootTimeHistogram metrics.Histogram `name:"boot_time_duration"`

	Logger log.Logger

	Address string
	Auth    acquire.Acquirer
}

var destinationRegex = regexp.MustCompile(`^(?P<event>[^\/]+)\/((?P<prefix>(?i)mac|uuid|dns|serial):(?P<id>[^\/]+))\/(?P<type>[^\/\s]+)`)
var onlineRegex = regexp.MustCompile(".*/online$")
var offlineRegex = regexp.MustCompile(".*/offline$")

func (b BootTimeCalc) Parse(msg wrp.Message) error {
	/**
	Steps to calculate boot time:
	1) Determine if message is online event
	2) Get lastest Offline event from Codex where metadata field of /boot-time differs of online event.
	3) Subtract Online birthdate from steps 2 event Birthdate
	4) Record Metric
	*/
	if !destinationRegex.MatchString(msg.Destination) || !onlineRegex.MatchString(msg.Destination) {
		logging.Debug(b.Logger).Log(logging.MessageKey(), "event is not an online event")
		return nil
	}
	bootTimeInt, err := GetWRPBootTime(msg)
	if err != nil {
		return err
	}
	bootTime := time.Unix(bootTimeInt, 0)

	matchSub := destinationRegex.FindStringSubmatch(msg.Destination)
	deviceID := matchSub[2]
	events := getEvents(deviceID, b.Logger, b.Address, b.Auth)

	latestBootTime := bootTimeInt
	previousBootTime := int64(0)
	for _, event := range events {
		if onlineRegex.MatchString(event.Dest) {
			eventBootTimeInt, err := GetEventBootTime(event)
			if err != nil {
				continue
			}

			if eventBootTimeInt > latestBootTime {
				logging.Error(b.Logger).Log(logging.MessageKey(), "found newer boot-time")
				return nil
			}
			if eventBootTimeInt == latestBootTime && event.TransactionUUID != msg.TransactionUUID {
				logging.Error(b.Logger).Log(logging.MessageKey(), "found same boot-time")
				return nil
			}
			if eventBootTimeInt > previousBootTime {
				previousBootTime = eventBootTimeInt
			}
		}
	}

	lastestBirthDate := int64(0)
	for _, event := range events {
		if offlineRegex.MatchString(event.Dest) {
			eventBootTimeInt, err := GetEventBootTime(event)
			if err != nil {
				continue
			}

			if eventBootTimeInt == previousBootTime {
				if event.BirthDate > lastestBirthDate {
					lastestBirthDate = event.BirthDate
				}
			}
		}
	}

	restartTime := math.Abs(time.Unix(0, lastestBirthDate).Sub(bootTime).Seconds())

	if lastestBirthDate != int64(0) && previousBootTime != int64(0) {
		b.BootTimeHistogram.With(HardwareLabel, msg.Metadata[hardwareKey], FirmwareLabel, msg.Metadata[firmwareKey]).Observe(restartTime)
	} else {
		logging.Error(b.Logger).Log(logging.MessageKey(), "failed to get restart time")
	}

	return nil
}

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

type ResponseEvent struct {
	Data []Event
}

func GetWRPBootTime(msg wrp.Message) (int64, error) {
	var bootTime int64
	var err error
	if bootTimeStr, ok := msg.Metadata[bootTimeKey]; ok {
		bootTime, err = strconv.ParseInt(bootTimeStr, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return bootTime, nil
}
func GetEventBootTime(msg Event) (int64, error) {
	var bootTime int64
	var err error
	if bootTimeStr, ok := msg.Metadata[bootTimeKey]; ok {
		bootTime, err = strconv.ParseInt(bootTimeStr, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return bootTime, nil
}

func getEvents(device string, logger log.Logger, codexAddress string, codexAuth acquire.Acquirer) []Event {
	eventList := make([]Event, 0)
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/device/%s/events", codexAddress, device), nil)
	if err != nil {
		logging.Error(logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to create request")
		return eventList
	}

	auth, err := codexAuth.Acquire()
	if err != nil {
		logging.Error(logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to get codex auth")
	}

	request.Header.Add("Authorization", auth)

	status, data, err := doRequest(request, logger)
	if err != nil {
		logging.Error(logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to complete request")
		return eventList
	}

	if status != 200 {
		logging.Error(logger).Log("status", status, logging.MessageKey(), "non 200", "url", request.URL)
		return eventList
	}

	err = json.Unmarshal(data, &eventList)
	if err != nil {
		logging.Error(logger).Log(logging.ErrorKey(), err, logging.MessageKey(), "failed to read body")
		return eventList
	}
	return eventList
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
