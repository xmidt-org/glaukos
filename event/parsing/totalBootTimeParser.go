/**
 *  Copyright (c) 2021  Comcast Cable Communications Management, LLC
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
)

// RebootTimeParser takes fully-manageable events and calculates the reboot time of a device by getting the last
// reboot-pending event from codex.
type RebootTimeParser struct {
	Measures Measures
	Logger   log.Logger
	Client   EventClient
	Label    string
}

var (
	errEventNotFound = errors.New("event not found")
)

var manageableRegex = regexp.MustCompile(".*/fully-manageable/")
var rebootRegex = regexp.MustCompile(".*/reboot-pending/")

/* Parse calculates reboot time of devices by querying codex for the latest device events and performing
 calculations. An analysis of codex events is only triggered by a fully-manageable event from caduceus.
 Steps to calculate boot time:
	 1) Determine if message is fully-manageable event.
	 2) Get latest reboot-pending event from Codex where metadata field of /boot-time differs from the fully-manageable event.
	 3) Subtract fully-manageable's birthdate from step 2's event birthdate.
	 4) Record Metric.
*/
func (b *RebootTimeParser) Parse(wrpWithTime queue.WrpWithTime) error {
	// Add to metrics if no error calculating restart time.
	if restartTime, err := b.calculateRestartTime(wrpWithTime); err == nil && restartTime > 0 {
		b.Measures.RebootTimeHistogram.With(HardwareLabel, wrpWithTime.Message.Metadata[hardwareKey], FirmwareLabel, wrpWithTime.Message.Metadata[firmwareKey]).Observe(restartTime)
	}

	return nil
}

func (b *RebootTimeParser) calculateRestartTime(wrpWithTime queue.WrpWithTime) (float64, error) {
	msg := wrpWithTime.Message
	// If event is not an fully-manageable event, do not continue with calculations.
	if !destinationRegex.MatchString(msg.Destination) || !manageableRegex.MatchString(msg.Destination) {
		level.Debug(b.Logger).Log(xlog.MessageKey(), "event is not an fully-manageable event")
		return -1, nil
	}

	// Get boot time and device id from message.
	bootTimeInt, deviceID, err := getWRPInfo(destinationRegex, msg)
	if err != nil {
		level.Error(b.Logger).Log(xlog.ErrorKey(), err)
		return -1, err
	}

	// Get events from codex pertaining to this device id.
	events := b.Client.GetEvents(deviceID)
	latestPreviousEvent := Event{}

	// Go through events to find reboot-pending event with the boot-time of the previous session
	for _, event := range events {
		if latestPreviousEvent, err = checkLatestPreviousEvent(event, latestPreviousEvent, bootTimeInt, rebootRegex); err != nil {
			level.Error(b.Logger).Log(xlog.ErrorKey(), err, "parser name", b.Label, "deviceID", deviceID, "current event id", msg.TransactionUUID)
			if errors.Is(err, errNewerBootTime) {
				// Something is wrong with this event's boot time, we shouldn't continue.
				b.Measures.UnparsableEventsCount.With(ParserLabel, b.Label, ReasonLabel, eventBootTimeErr).Add(1.0)
				return -1, err
			}
		}
	}

	if valid, err := isEventValid(latestPreviousEvent, rebootRegex, time.Now); !valid {
		level.Error(b.Logger).Log(xlog.ErrorKey(), err, "parser name", b.Label, "deviceID", deviceID, "codex event destination", latestPreviousEvent.Dest)
		return -1, fmt.Errorf("%s: %w", "Invalid previous event found", err)
	}

	// boot time calculation
	// Event birthdate is saved in unix nanoseconds, so we must first convert it to a unix time using nanoseconds.
	restartTime := wrpWithTime.Beginning.Sub(time.Unix(0, latestPreviousEvent.BirthDate)).Seconds()

	if restartTime <= 0 {
		err = errInvalidRestartTime
		level.Error(b.Logger).Log(xlog.ErrorKey(), err, "restart time", restartTime, "current event birthdate", wrpWithTime.Beginning.UnixNano(), "codex event birthdate", latestPreviousEvent.BirthDate)
		return -1, err
	}

	return restartTime, nil
}

// sees if this event is part of the most recent previous session
func checkLatestPreviousEvent(e Event, previousEventTracked Event, latestBootTime int64, eventType *regexp.Regexp) (Event, error) {
	eventBootTimeInt, err := GetEventBootTime(e)
	previousEventBootTime, _ := GetEventBootTime(previousEventTracked)

	if err != nil {
		return previousEventTracked, err
	}

	if eventBootTimeInt > latestBootTime {
		return Event{}, fmt.Errorf("%w. Codex Event: %s", errNewerBootTime, e.TransactionUUID)
	}

	// If this event has a boot time greater than what we've seen so far
	// but still less than the current boot-time, it means that this event
	// is part of a more recent previous cycle.
	if eventBootTimeInt > previousEventBootTime && eventBootTimeInt < latestBootTime {
		return e, nil
	}

	// If the event is of the type we are looking for and has the same boot-time as the
	// newest previous boot time, return this event.
	if eventBootTimeInt == previousEventBootTime && eventType.MatchString(e.Dest) {
		// If the previously tracked event doesn't match the event we're looking for,
		// return the current event. If both events match the type of event we are looking for,
		// compare the birthdates and return the one with the older birthdate.
		if !eventType.MatchString(previousEventTracked.Dest) || e.BirthDate < previousEventTracked.BirthDate {
			return e, nil
		}
	}
	return previousEventTracked, nil
}

func isEventValid(event Event, expectedType *regexp.Regexp, currTime func() time.Time) (bool, error) {
	// see if event found matches expected event type
	if !expectedType.MatchString(event.Dest) {
		return false, fmt.Errorf("%w. Type Expected: %s. Type Found: %s", errEventNotFound, expectedType.String(), event.Dest)
	}

	// check if boot-time is valid
	bootTime, err := GetEventBootTime(event)
	if bootTime <= 0 {
		var parsingErr error
		if err != nil {
			parsingErr = err
		}
		return false, fmt.Errorf("%w. Parsed boot-time: %d, parsing err: %v", errPastDate, bootTime, parsingErr)
	}

	if valid, err := isDateValid(currTime, 12*time.Hour, time.Hour, time.Unix(bootTime, 0)); !valid {
		return false, fmt.Errorf("Invalid boot-time: %w", err)
	}

	// see if birthdate is valid
	if valid, err := isDateValid(currTime, 12*time.Hour, time.Hour, time.Unix(0, event.BirthDate)); !valid {
		return false, fmt.Errorf("Invalid birthdate: %w", err)
	}

	return true, nil
}
