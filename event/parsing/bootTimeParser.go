/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package parsing

import (
	"errors"
	"math"
	"regexp"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/xmidt-org/webpa-common/logging"
	"github.com/xmidt-org/wrp-go/v3"
)

const (
	hardwareKey = "/hw-model"
	firmwareKey = "/fw-name"
	bootTimeKey = "/boot-time"

	bootTimeParserLabel = "boot_time_parser"
	eventBootTimeErr    = "event_boot_time_err"
)

type EventClient interface {
	GetEvents(string) []Event
}

// BootTimeParser takes online events and calculates the reboot time of a device by getting the last
// offline event from codex.
type BootTimeParser struct {
	BootTimeHistogram     metrics.Histogram `name:"boot_time_duration"`
	UnparsableEventsCount metrics.Counter   `name:"unparsable_events_count"`
	Logger                log.Logger
	Client                EventClient
}

var destinationRegex = regexp.MustCompile(`^(?P<event>[^/]+)/((?P<prefix>(?i)mac|uuid|dns|serial):(?P<id>[^/]+))/(?P<type>[^/\s]+)`)
var onlineRegex = regexp.MustCompile(".*/online$")
var offlineRegex = regexp.MustCompile(".*/offline$")

/* Parse calculates boot time of devices by querying codex for the latest offline events and performing
calculations. An analysis of codex events is only triggered by a device online event from caduceus.
Steps to calculate boot time:
	1) Determine if message is online event.
	2) Get lastest Offline event from Codex where metadata field of /boot-time differs of online event.
	3) Subtract Online birthdate from steps 2 event Birthdate.
	4) Record Metric.
*/
func (b BootTimeParser) Parse(msg wrp.Message) error {
	// add to metrics if no error calculating restart time
	if restartTime, err := b.calculateRestartTime(msg); err == nil && restartTime > 0 {
		b.BootTimeHistogram.With(HardwareLabel, msg.Metadata[hardwareKey], FirmwareLabel, msg.Metadata[firmwareKey]).Observe(restartTime)
	}

	return nil
}

func (b *BootTimeParser) calculateRestartTime(msg wrp.Message) (float64, error) {
	// if event is not an online event, do not continue with calculations
	if !destinationRegex.MatchString(msg.Destination) || !onlineRegex.MatchString(msg.Destination) {
		logging.Debug(b.Logger).Log(logging.MessageKey(), "event is not an online event")
		return -1, nil
	}

	// get boot time and device id from message
	bootTimeInt, deviceID, err := getWRPInfo(destinationRegex, msg)
	if err != nil {
		logging.Error(b.Logger).Log(logging.MessageKey(), err)
		return -1, err
	}

	latestBootTime := time.Unix(bootTimeInt, 0)
	previousBootTime := int64(0)

	// get events from codex pertaining to this device id
	events := b.Client.GetEvents(deviceID)

	// find the previous boot-time and make sure that the boot time we have is the latest one
	for _, event := range events {
		if previousBootTime, err = checkOnlineEvent(event, msg.TransactionUUID, previousBootTime, bootTimeInt); err != nil {
			logging.Error(b.Logger).Log(logging.MessageKey(), err)
			if previousBootTime < 0 {
				// something is wrong with this event's boot time, we shouldn't continue
				b.UnparsableEventsCount.With(ParserLabel, bootTimeParserLabel, ReasonLabel, eventBootTimeErr).Add(1.0)
				return -1, err
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
	// event birthdate is saved in unix nanoseconds, so we must first convert it to a unix time using nanoseconds
	restartTime := math.Abs(latestBootTime.Sub(time.Unix(0, latestOfflineEvent)).Seconds())
	// add to metrics or log the error
	if latestOfflineEvent != 0 && previousBootTime != 0 {
		return restartTime, nil
	}

	err = errors.New("failed to get restart time")
	logging.Error(b.Logger).Log(logging.MessageKey(), err)
	return -1, err

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

	// if we find a more recent boot time that is not the boot time we are currently comparing
	if eventBootTimeInt > previousBootTime && e.TransactionUUID != currentUUID {
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
