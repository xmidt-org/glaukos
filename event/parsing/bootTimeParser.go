/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package parsing

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/wrp-go/v3"
)

const (
	hardwareKey = "/hw-model"
	firmwareKey = "/fw-name"
	bootTimeKey = "/boot-time"

	bootTimeParserLabel = "boot_time_parser"
	eventBootTimeErr    = "event_boot_time_err"
)

var (
	errNewerBootTime      = errors.New("found newer boot-time")
	errSameBootTime       = errors.New("found same boot-time")
	errRestartTime        = errors.New("failed to get restart time")
	errInvalidRestartTime = errors.New("invalid restart time")
)

type EventClient interface {
	GetEvents(string) []Event
}

// BootTimeParser takes online events and calculates the reboot time of a device by getting the last
// offline event from codex.
type BootTimeParser struct {
	Measures Measures
	Logger   log.Logger
	Client   EventClient
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
func (b *BootTimeParser) Parse(wrpWithTime queue.WrpWithTime) error {
	// Add to metrics if no error calculating restart time.
	if restartTime, err := b.calculateRestartTime(wrpWithTime); err == nil && restartTime > 0 {
		b.Measures.BootTimeHistogram.With(HardwareLabel, wrpWithTime.Message.Metadata[hardwareKey], FirmwareLabel, wrpWithTime.Message.Metadata[firmwareKey]).Observe(restartTime)
	}

	return nil
}

func (b *BootTimeParser) calculateRestartTime(wrpWithTime queue.WrpWithTime) (float64, error) {
	msg := wrpWithTime.Message
	// If event is not an online event, do not continue with calculations.
	if !destinationRegex.MatchString(msg.Destination) || !onlineRegex.MatchString(msg.Destination) {
		level.Debug(b.Logger).Log(xlog.MessageKey(), "event is not an online event")
		return -1, nil
	}

	// Get boot time and device id from message.
	bootTimeInt, deviceID, err := getWRPInfo(destinationRegex, msg)
	if err != nil {
		level.Error(b.Logger).Log(xlog.ErrorKey(), err)
		return -1, err
	}

	previousBootTime := int64(0)

	// Get events from codex pertaining to this device id.
	events := b.Client.GetEvents(deviceID)

	// Find the previous boot-time and make sure that the boot time we have is the latest one.
	for _, event := range events {
		if previousBootTime, err = checkOnlineEvent(event, msg.TransactionUUID, previousBootTime, bootTimeInt); err != nil {
			level.Error(b.Logger).Log(xlog.ErrorKey(), err, "parser name", bootTimeParserLabel, "deviceID", deviceID, "current event id", msg.TransactionUUID)
			if previousBootTime < 0 {
				// Something is wrong with this event's boot time, we shouldn't continue.
				b.Measures.UnparsableEventsCount.With(ParserLabel, bootTimeParserLabel, ReasonLabel, eventBootTimeErr).Add(1.0)
				return -1, err
			}
		}

	}

	// Look through offline events and find the latest offline event.
	latestOfflineEvent := int64(0)
	for _, event := range events {
		if latestOfflineEvent, err = checkOfflineEvent(event, previousBootTime, latestOfflineEvent); err != nil {
			level.Error(b.Logger).Log(xlog.ErrorKey(), err)
		}
	}

	// boot time calculation
	// Event birthdate is saved in unix nanoseconds, so we must first convert it to a unix time using nanoseconds.
	restartTime := wrpWithTime.Beginning.Sub(time.Unix(0, latestOfflineEvent)).Seconds()

	if restartTime <= 0 {
		err = errInvalidRestartTime
		level.Error(b.Logger).Log(xlog.ErrorKey(), err, "Restart time", restartTime)
		return -1, err
	}
	// Add to metrics or log the error.
	if latestOfflineEvent != 0 && previousBootTime != 0 {
		return restartTime, nil
	}

	err = errRestartTime
	level.Error(b.Logger).Log(xlog.ErrorKey(), err)
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
		return -1, fmt.Errorf("%w. Codex Event: %s", errNewerBootTime, e.TransactionUUID)
	}

	// if another online event is found with the same boot time but different transaction uuid, it means the event
	// received is not the result of a true restart
	if eventBootTimeInt == latestBootTime && e.TransactionUUID != currentUUID {
		return -1, fmt.Errorf("%w. Codex Event: %s", errSameBootTime, e.TransactionUUID)
	}

	// If we find a more recent boot time that is not the boot time we are currently comparing, return the boot time.
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
