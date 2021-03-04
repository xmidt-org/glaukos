package metricparsers

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

type Config struct {
	Name          string
	StartingEvent parsing.EventConfig
	EndingEvent   parsing.EventConfig
}

type TimeElapsedParser struct {
	measures      Measures
	logger        log.Logger
	startingEvent parsing.EventValidator
	endingEvent   parsing.EventValidator
	client        parsing.EventClient
	label         string
}

var eventRegex = regexp.MustCompile(`^(?P<event>[^/]+)/((?P<prefix>(?i)mac|uuid|dns|serial):(?P<id>[^/]+))/(?P<type>[^/\s]+)`)

/* Parse calculates the difference of events (either by boot-time or birthdate, depending on what is configured)
 by querying codex for the latest device events and performing calculations.
 An analysis of codex events is only triggered by an incoming event from caduceus that matches the destination regex
 of the ending event.
 Steps to calculate:
	 1) Determine if message is an ending event.
	 2) Get latest starting event from Codex where metadata field of /boot-time differs from the ending event's.
	 3) Subtract ending event's birthdate or boot-time (depending on configuration) from step 2's event birthdate or boot-time (depending on config).
	 4) Record Metric.
*/
func (t *TimeElapsedParser) Parse(wrpWithTime queue.WrpWithTime) error {
	// Add to metrics if no error calculating restart time.
	if restartTime, err := t.calculateRestartTime(wrpWithTime); err == nil && restartTime > 0 {
		b.measures.BootTimeHistograms[t.label].With(HardwareLabel, wrpWithTime.Message.Metadata[hardwareKey], FirmwareLabel, wrpWithTime.Message.Metadata[firmwareKey]).Observe(restartTime)
	} else {
		return err
	}

	return nil
}

func (t *TimeElapsedParser) calculateRestartTime(wrpWithTime queue.WrpWithTime) (float64, error) {
	msg := wrpWithTime.Message
	// If event is not an fully-manageable event, do not continue with calculations.
	if !eventRegex.MatchString(msg.Destination) || !t.endingEvent.regex.MatchString(msg.Destination) {
		level.Debug(b.Logger).Log(xlog.MessageKey(), "event does not match event type",
			"desired type", t.endingEvent.regex.String(), "message destination", msg.Destination)
		return -1, nil
	}

	// Get boot time and device id from message.
	bootTimeInt, deviceID, err := getWRPInfo(eventRegex, msg)
	if err != nil {
		level.Error(b.Logger).Log(xlog.ErrorKey(), err)
		return -1, err
	}

	// Get events from codex pertaining to this device id.
	events := b.Client.GetEvents(deviceID)
	latestPreviousEvent := Event{}

	// Go through events to find a starting event with the boot-time of the previous session
	for _, event := range events {
		if latestPreviousEvent, err = checkLatestPreviousEvent(event, latestPreviousEvent, bootTimeInt, t.startingEvent.regex); err != nil {
			level.Error(t.Logger).Log(xlog.ErrorKey(), err)
			if errors.Is(err, errNewerBootTime) {
				// Something is wrong with this event's boot time, we shouldn't continue.
				t.measures.UnparsableEventsCount.With(ParserLabel, t.Label, ReasonLabel, eventBootTimeErr).Add(1.0)
				return -1, err
			}
		}
	}

	if valid, err := isEventValid(latestPreviousEvent, t.startingEvent.regex, time.Now); !valid {
		level.Error(b.Logger).Log(xlog.ErrorKey(), err)
		return -1, fmt.Errorf("%s: %w", "Invalid previous event found", err)
	}

	var endingTime time.Time
	var startingTime time.Time
	// TODO: rework
	if t.startingEvent.CalculateUsing == parsing.BirthDate {
		startingTime = wrpWithTime.Beginning
	} else {
		startingTime = wrpWithTime.Message
	}

	// boot time calculation
	// Event birthdate is saved in unix nanoseconds, so we must first convert it to a unix time using nanoseconds.
	restartTime := wrpWithTime.Beginning.Sub(time.Unix(0, latestPreviousEvent.BirthDate)).Seconds()

	if restartTime <= 0 {
		err = errInvalidRestartTime
		level.Error(b.Logger).Log(xlog.ErrorKey(), err, "restart time", restartTime)
		return -1, err
	}

	return restartTime, nil
}
