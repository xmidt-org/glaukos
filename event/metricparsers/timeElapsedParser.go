package metricparsers

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/wrp-go/v3"
)

type EventClient interface {
	GetEvents(string) []client.Event
}

type TimeElapsedParser struct {
	measures         Measures
	logger           log.Logger
	initialValidator parsing.EventValidation
	endValidator     parsing.EventValidation
	client           EventClient
	label            string
}

var (
	errNewerBootTime      = errors.New("found newer boot-time")
	errSameBootTime       = errors.New("found same boot-time")
	errInvalidTimeElapsed = errors.New("amount of time elapsed is invalid")
	errInvalidBootTime    = errors.New("invalid boot time")
	errInvalidDeviceID    = errors.New("could not parse device id")
	errInvalidEventDest   = errors.New("event destination does not match event regex")

	errInvalidPrevEvent = errors.New("invalid previous event found")
)

const (
	defaultName = "time_elapsed_parser"
	hardwareKey = "/hw-model"
	firmwareKey = "/fw-name"

	errEventBootTime        = "err_event_boot_time"
	errNoFirmwareOrHardware = "err_no_firmware_or_hardware"
)

var eventRegex = regexp.MustCompile(`^(?P<event>[^/]+)/((?P<prefix>(?i)mac|uuid|dns|serial):(?P<id>[^/]+))/(?P<type>[^/\s]+)`)

// CreateNewTimeElapsedParser creates a new TimeElapsedParser from a TimeElapsedConfig
func CreateNewTimeElapsedParser(config TimeElapsedConfig, name string, eventClient EventClient, logger log.Logger, measures Measures) (*TimeElapsedParser, error) {
	initialValidator, err := parsing.NewEventValidation(config.InitialEvent, time.Hour, time.Now)
	if err != nil {
		return nil, err
	}

	endValidator, err := parsing.NewEventValidation(config.IncomingEvent, time.Hour, time.Now)
	if err != nil {
		return nil, err
	}

	return &TimeElapsedParser{
		measures:         measures,
		logger:           GetParserLogger(logger, name),
		client:           eventClient,
		initialValidator: initialValidator,
		endValidator:     endValidator,
		label:            name,
	}, nil
}

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
	if restartTime, err := t.calculateTimeElapsed(wrpWithTime); err == nil && restartTime > 0 {
		hardwareVal, hardwareFound := parsing.GetMetadataValue(hardwareKey, wrpWithTime.Message.Metadata)
		firmwareVal, firmwareFound := parsing.GetMetadataValue(firmwareKey, wrpWithTime.Message.Metadata)
		if hardwareFound && firmwareFound {
			histogram, ok := t.measures.TimeElapsedHistograms[t.label]
			if ok {
				histogram.With(HardwareLabel, hardwareVal, FirmwareLabel, firmwareVal).Observe(restartTime)
			} else {
				level.Error(t.logger).Log(xlog.ErrorKey(), "No histogram found for this time elapsed parser")
			}
		} else {
			t.measures.UnparsableEventsCount.With(ParserLabel, t.label, ReasonLabel, errNoFirmwareOrHardware).Add(1.0)
		}
	} else if err != nil {
		return err
	}

	return nil
}

// Name implements the Parser interface, returning the name associated with the
// TimeElapsedParser
func (t *TimeElapsedParser) Name() string {
	return t.label
}

func (t *TimeElapsedParser) calculateTimeElapsed(wrpWithTime queue.WrpWithTime) (float64, error) {
	msg := wrpWithTime.Message
	valid, err := t.endValidator.IsWRPValid(wrpWithTime)
	// If event is not of the intended type, do not continue with calculations.
	if !eventRegex.MatchString(msg.Destination) {
		level.Debug(t.logger).Log(xlog.MessageKey(), "incoming event does not match event regex", "destination", msg.Destination)
		return -1, errInvalidEventDest
	}

	if !valid {
		level.Debug(t.logger).Log(xlog.MessageKey(), "incoming event is not valid", "error", err)
		return -1, nil
	}

	// Get boot time and device id from message.
	bootTimeInt, deviceID, err := getWRPInfo(eventRegex, msg)
	if err != nil {
		level.Error(t.logger).Log(xlog.ErrorKey(), err)
		return -1, err
	}

	// Get events from codex pertaining to this device id.
	events := t.client.GetEvents(deviceID)
	latestPreviousEvent := client.Event{}

	// Go through events to find a starting event with the boot-time of the previous session
	for _, event := range events {
		if latestPreviousEvent, err = t.checkLatestInitialEvent(event, latestPreviousEvent, bootTimeInt); err != nil {
			t.logErrWithEventDetails(err, deviceID, msg, event)
			if errors.Is(err, errNewerBootTime) || errors.Is(err, errSameBootTime) {
				// Something is wrong with this event's boot time, we shouldn't continue.
				t.measures.UnparsableEventsCount.With(ParserLabel, t.label, ReasonLabel, errEventBootTime).Add(1.0)
				return -1, err
			}
		}
	}

	if valid, err := t.initialValidator.IsEventValid(latestPreviousEvent); !valid {
		previousEventErr := fmt.Errorf("%w: %v", errInvalidPrevEvent, err)
		t.logErrWithEventDetails(previousEventErr, deviceID, msg, latestPreviousEvent)
		return -1, previousEventErr
	}

	// errors will not pop up here because we have already checked that both events are valid earlier
	startingTime, _ := t.initialValidator.GetEventCompareTime(latestPreviousEvent)
	endingTime, _ := t.endValidator.GetWRPCompareTime(wrpWithTime)

	// calculate difference
	timeElapsed := endingTime.Sub(startingTime).Seconds()

	if timeElapsed <= 0 {
		err = errInvalidTimeElapsed
		level.Error(t.logger).Log(xlog.ErrorKey(), err, "time calculated", timeElapsed)
		return -1, err
	}

	return timeElapsed, nil
}

// sees if this event is part of the most recent previous session
func (t *TimeElapsedParser) checkLatestInitialEvent(e client.Event, previousEventTracked client.Event, latestBootTime int64) (client.Event, error) {
	eventBootTimeInt, err := parsing.GetEventBootTime(e)
	previousEventBootTime, _ := parsing.GetEventBootTime(previousEventTracked)

	if err != nil {
		return previousEventTracked, fmt.Errorf("%w. %v", errInvalidBootTime, err)
	}

	if eventBootTimeInt <= 0 {
		return previousEventTracked, errInvalidBootTime
	}

	if eventBootTimeInt > latestBootTime {
		return client.Event{}, errNewerBootTime
	}

	// If this event has a boot time greater than what we've seen so far
	// but still less than the current boot-time, it means that this event
	// is part of a more recent previous cycle.
	if eventBootTimeInt > previousEventBootTime && eventBootTimeInt < latestBootTime {
		return e, nil
	}

	// If the event is of the type we are looking for and has the same boot-time as the
	// newest previous boot time.
	if eventBootTimeInt == previousEventBootTime && t.initialValidator.ValidateType(e.Dest) {
		// If events with the same boot-time for this event type is not allowed, return an error
		if !t.initialValidator.DuplicateAllowed() && t.initialValidator.ValidateType(previousEventTracked.Dest) {
			return client.Event{}, errSameBootTime
		}

		// If the previously tracked event doesn't match the event we're looking for,
		// return the current event. If both events match the type of event we are looking for,
		// compare the birthdates and return the one with the older birthdate.
		if !t.initialValidator.ValidateType(previousEventTracked.Dest) || e.BirthDate < previousEventTracked.BirthDate {
			return e, nil
		}
	}

	return previousEventTracked, nil
}

func (t *TimeElapsedParser) logErrWithEventDetails(err error, deviceID string, incomingWRP wrp.Message, codexEvent client.Event) {
	level.Error(t.logger).Log(xlog.ErrorKey(), err, "deviceID", deviceID, "current event", incomingWRP.TransactionUUID, "codex event", codexEvent.TransactionUUID)
}

func getWRPInfo(destinationRegex *regexp.Regexp, msg wrp.Message) (int64, string, error) {
	deviceID, err := parsing.GetDeviceID(destinationRegex, msg.Destination)
	if err != nil {
		return 0, "", fmt.Errorf("%w. %v", errInvalidDeviceID, err)
	}

	bootTime, err := parsing.GetWRPBootTime(msg)
	if err != nil {
		return 0, "", fmt.Errorf("%w. %v", errInvalidBootTime, err)
	}

	return bootTime, deviceID, nil
}
