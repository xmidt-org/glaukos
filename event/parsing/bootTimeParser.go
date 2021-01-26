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

	bootTimeParserLabel = "boot_time_parser"
	eventBootTimeErr    = "event_boot_time_err"
)

// BootTimeParser takes online events and calculates the reboot time of a device by getting the last
// offline event from codex.
type BootTimeParser struct {
	BootTimeHistogram     metrics.Histogram `name:"boot_time_duration"`
	UnparsableEventsCount metrics.Counter   `name:"unparsable_events_count"`
	Logger                log.Logger

	Address string
	Auth    acquire.Acquirer
}

var destinationRegex = regexp.MustCompile(`^(?P<event>[^/]+)/((?P<prefix>(?i)mac|uuid|dns|serial):(?P<id>[^/]+))/(?P<type>[^/\s]+)`)
var onlineRegex = regexp.MustCompile(".*/online$")
var offlineRegex = regexp.MustCompile(".*/offline$")

/* Parse calculates boot time of devices by querying codex for the latest offline events and performing
calculations. An analysis of codex events is only triggered by a device online event from caduceus.
Steps to calculate boot time:
	1) Determine if message is online event
	2) Get lastest Offline event from Codex where metadata field of /boot-time differs of online event.
	3) Subtract Online birthdate from steps 2 event Birthdate
	4) Record Metric
*/
func (b BootTimeParser) Parse(msg wrp.Message) error {
	// if event is not an online event, do not continue with calculations
	if !destinationRegex.MatchString(msg.Destination) || !onlineRegex.MatchString(msg.Destination) {
		logging.Debug(b.Logger).Log(logging.MessageKey(), "event is not an online event")
		return nil
	}

	// get boot time and device id from message
	bootTimeInt, deviceID, err := getWRPInfo(destinationRegex, msg)
	if err != nil {
		return err
	}

	latestBootTime := time.Unix(bootTimeInt, 0)
	previousBootTime := int64(0)

	// get events from codex pertaining to this device id
	events := getEvents(deviceID, b.Logger, b.Address, b.Auth)

	// find the previous boot-time and make sure that the boot time we have is the latest one
	for _, event := range events {
		if previousBootTime, err = checkOnlineEvent(event, msg.TransactionUUID, previousBootTime, bootTimeInt); err != nil {
			logging.Error(b.Logger).Log(logging.MessageKey(), err)
			if previousBootTime < 0 {
				// something is wrong with this event's boot time, we shouldn't continue
				b.UnparsableEventsCount.With(ParserLabel, bootTimeParserLabel, ReasonLabel, eventBootTimeErr).Add(1.0)
				return nil
			}
		}
	}

	// look through offline events and find the latest offline event
	latestOfflineEvent := int64(0)
	for _, event := range events {
		if latestOfflineEvent, err = checkOfflineEvent(event, previousBootTime, latestOfflineEvent); err != nil {
			logging.Error(b.Logger).Log(logging.MessageKey(), err)
		}
	}

	// boot time calculation
	restartTime := math.Abs(time.Unix(0, latestOfflineEvent).Sub(latestBootTime).Seconds())

	// add to metrics or log the error
	if latestOfflineEvent != 0 && previousBootTime != 0 {
		b.BootTimeHistogram.With(HardwareLabel, msg.Metadata[hardwareKey], FirmwareLabel, msg.Metadata[firmwareKey]).Observe(restartTime)
	} else {
		logging.Error(b.Logger).Log(logging.MessageKey(), "failed to get restart time")
	}

	return nil
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

type ResponseEvent struct {
	Data []Event
}

// query codex for events related to a device
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

// Checks an event and sees if it is an online event.
// If event is an online event, checks for the boot time to see if it is greater than previousBootTime.
// Returns either the event's boot time or the previous boot time, whichever is greater.
// In cases where the event's boot time is found to be equal or greater to the latest boot time, we return -1 and error, indicating
// that we should not continue to parse metrics from this event.
func checkOnlineEvent(e Event, currentUUID string, previousBootTime int64, latestBootTime int64) (int64, error) {
	if !onlineRegex.MatchString(e.Dest) {
		return previousBootTime, nil
	}

	eventBootTimeInt, err := GetEventBootTime(e)
	if err != nil {
		return previousBootTime, err
	}

	if eventBootTimeInt > latestBootTime {
		return -1, errors.New("found newer boot-time")
	}
	if eventBootTimeInt == latestBootTime && e.TransactionUUID != currentUUID {
		return -1, errors.New("found same boot-time")
	}
	if eventBootTimeInt > previousBootTime {
		return eventBootTimeInt, nil
	}

	return previousBootTime, nil
}

// Checks an event and sees if it is an offline event.
// If event is an offline event, checks for the boot time to see if it matches the boot time we are looking for.
// Returns either the event's birthdate or the latest birth date found, whichever is greater.
func checkOfflineEvent(e Event, previousBootTime int64, latestBirthDate int64) (int64, error) {
	if !offlineRegex.MatchString(e.Dest) {
		return latestBirthDate, nil
	}

	eventBootTimeInt, err := GetEventBootTime(e)
	if err != nil {
		return latestBirthDate, err
	}

	if eventBootTimeInt == previousBootTime {
		if e.BirthDate > latestBirthDate {
			return e.BirthDate, nil
		}
	}

	return latestBirthDate, nil
}

func getWRPInfo(destinationRegex *regexp.Regexp, msg wrp.Message) (bootTime int64, deviceID string, err error) {
	bootTime, err = GetWRPBootTime(msg)
	if err != nil {
		return
	}

	deviceID, err = GetDeviceID(destinationRegex, msg.Destination)
	if err != nil {
		return
	}

	return
}
