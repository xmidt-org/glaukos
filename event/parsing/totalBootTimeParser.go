/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package parsing

import (
	"math"
	"regexp"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/wrp-go/v3"
)

// TotalBootTimeParser takes fully-manageabke events and calculates the reboot time of a device by getting the last
// reboot-pending event from codex.
type TotalBootTimeParser struct {
	Measures Measures
	Logger   log.Logger
	Client   EventClient
	Label    string
}

var manageableRegex = regexp.MustCompile(".*/fully-manageable/")
var rebootRegex = regexp.MustCompile(".*/reboot-pending/")

/* Parse calculates boot time of devices by querying codex for the latest device events and performing
 calculations. An analysis of codex events is only triggered by a fully-manageable event from caduceus.
 Steps to calculate boot time:
	 1) Determine if message is fully-manageable event.
	 2) Get latest reboot-pending event from Codex where metadata field of /boot-time differs from the fully-manageable event.
	 3) Subtract fully-manageable's birthdate from step 2's event birthdate.
	 4) Record Metric.
*/
func (b TotalBootTimeParser) Parse(msg wrp.Message) error {
	// Add to metrics if no error calculating restart time.
	if restartTime, err := b.calculateRestartTime(msg); err == nil && restartTime > 0 {
		b.Measures.TotalBootTimeHistogram.With(HardwareLabel, msg.Metadata[hardwareKey], FirmwareLabel, msg.Metadata[firmwareKey]).Observe(restartTime)
	}

	return nil
}

func (b *TotalBootTimeParser) calculateRestartTime(msg wrp.Message) (float64, error) {
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

	previousBootTime := int64(0)
	latestRebootEvent := int64(0)
	// TODO: this should be the time that the request is received
	currentBirthDate := time.Now()

	// Get events from codex pertaining to this device id.
	events := b.Client.GetEvents(deviceID)

	// Find the previous boot-time and make sure that the boot time we have is the latest one.
	for _, event := range events {
		if previousBootTime, newBootTimeFound, err := getPreviousBootTime(event, previousBootTime, bootTimeInt, msg); err != nil {
			level.Error(b.Logger).Log(xlog.ErrorKey(), err)
			if previousBootTime < 0 {
				// Something is wrong with this event's boot time, we shouldn't continue.
				b.Measures.UnparsableEventsCount.With(ParserLabel, b.Label, ReasonLabel, eventBootTimeErr).Add(1.0)
				return -1, err
			}
		} else if newBootTimeFound {
			latestRebootEvent, _ = getLastRebootEvent(event, latestRebootEvent)
		}
	}

	// boot time calculation
	// Event birthdate is saved in unix nanoseconds, so we must first convert it to a unix time using nanoseconds.
	restartTime := math.Abs(currentBirthDate.Sub(time.Unix(0, latestRebootEvent)).Seconds())
	// Return the restart time or log the error.
	if latestRebootEvent != 0 {
		return restartTime, nil
	}

	err = errRestartTime
	level.Error(b.Logger).Log(xlog.ErrorKey(), err)
	return -1, err

}

// Get the latest previous boot time. Returns the latest previous boot time found, a boolean indicating if a more recent boot-time
// has been found, and a possible error
func getPreviousBootTime(e Event, previousBootTime int64, latestBootTime int64, currentMsg wrp.Message) (int64, bool, error) {
	eventBootTimeInt, err := GetEventBootTime(e)
	if err != nil {
		return previousBootTime, false, err
	}

	if eventBootTimeInt > latestBootTime {
		return -1, false, errNewerBootTime
	}

	if eventBootTimeInt > previousBootTime && e.TransactionUUID != currentMsg.TransactionUUID {
		return eventBootTimeInt, true, nil
	}

	return previousBootTime, false, nil
}

// get the last reboot-pending event
func getLastRebootEvent(e Event, latestRebootBirthDate int64) (int64, error) {
	if rebootRegex.MatchString(e.Dest) && e.BirthDate > latestRebootBirthDate {
		return e.BirthDate, nil
	}
	return latestRebootBirthDate, nil
}
