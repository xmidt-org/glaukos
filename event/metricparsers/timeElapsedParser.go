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
	measures          Measures
	logger            log.Logger
	initialValidator  parsing.EventValidation
	endValidator      parsing.EventValidation
	client            EventClient
	searchPastSession bool
	discardDuplicate  bool
	label             string
}

var (
	errNewerBootTime      = errors.New("found newer boot-time")
	errDuplicateFound     = errors.New("duplicate found")
	errInvalidTimeElapsed = errors.New("amount of time elapsed is invalid")
	errInvalidBootTime    = errors.New("invalid boot time")
	errInvalidDeviceID    = errors.New("could not parse device id")
	errInvalidEventDest   = errors.New("event destination does not match event regex")
	errWrongType          = errors.New("event type mismatch")
	errInvalidPrevEvent   = errors.New("invalid previous event found")
	errNoInitialEvent     = errors.New("no initial event found in codex")
)

const (
	defaultName = "time_elapsed_parser"
	hardwareKey = "/hw-model"
	firmwareKey = "/fw-name"

	errEventBootTime        = "err_event_boot_time"
	errNoFirmwareOrHardware = "err_no_firmware_or_hardware"
	errNoEventFound         = "err_no_initial_events"
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

func (t *TimeElapsedParser) validateIncomingMsg(wrpWithTime queue.WrpWithTime) (bool, error) {
	msg := wrpWithTime.Message
	valid, err := t.endValidator.IsWRPValid(wrpWithTime)
	// If event is not of the intended type, do not continue with calculations.
	if !eventRegex.MatchString(msg.Destination) {
		level.Debug(t.logger).Log(xlog.MessageKey(), "incoming event does not match event regex", "destination", msg.Destination)
		return false, errInvalidEventDest
	}

	if !valid {
		level.Debug(t.logger).Log(xlog.MessageKey(), "incoming event is not valid", "error", err)
		return false, errWrongType
	}

	return true, nil
}

func (t *TimeElapsedParser) getInitialBootTime(events []client.Event, currentBootTime int64) int64 {
	if !t.searchPastSession {
		return currentBootTime
	}

	var initialBootTime int64
	for _, event := range events {
		bootTime, _ := parsing.GetEventBootTime(event)
		if bootTime > initialBootTime && bootTime < currentBootTime {
			initialBootTime = bootTime
		}
	}

	return initialBootTime
}

func (t *TimeElapsedParser) calculateTimeElapsed(wrpWithTime queue.WrpWithTime) (float64, error) {
	if valid, err := t.validateIncomingMsg(wrpWithTime); !valid {
		return -1, err
	}

	msg := wrpWithTime.Message
	// Get boot time and device id from message.
	bootTimeInt, deviceID, err := getWRPInfo(eventRegex, msg)
	if err != nil {
		level.Error(t.logger).Log(xlog.ErrorKey(), err)
		return -1, err
	}

	// Get events from codex pertaining to this device id. Find the proper boot-time to find the initial event later.
	events := t.client.GetEvents(deviceID)
	initialBootTime := t.getInitialBootTime(events, bootTimeInt)
	if initialBootTime == 0 {
		level.Error(t.logger).Log(xlog.ErrorKey(), errNoInitialEvent)
		return -1, errNoInitialEvent
	}

	// Go through events to find a starting event
	latestPreviousEvent, err := t.findInitialEvent(events, msg, initialBootTime)
	if err != nil {
		if errors.Is(err, errNewerBootTime) || errors.Is(err, errDuplicateFound) {
			// Something is wrong with this event's boot time, we shouldn't continue.
			t.measures.UnparsableEventsCount.With(ParserLabel, t.label, ReasonLabel, errEventBootTime).Add(1.0)
			return -1, err
		}
	}

	if valid, err := t.initialValidator.IsEventValid(latestPreviousEvent); !valid {
		previousEventErr := fmt.Errorf("%w: %v", errInvalidPrevEvent, err)
		t.logErrWithEventDetails(previousEventErr, msg, latestPreviousEvent)
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

func (t *TimeElapsedParser) findInitialEvent(events []client.Event, incomingWRP wrp.Message, bootTimeSearch int64) (client.Event, error) {
	var latestEvent client.Event
	incomingBootTime, err := parsing.GetWRPBootTime(incomingWRP)
	if err != nil {
		return client.Event{}, errInvalidBootTime
	}

	for _, event := range events {
		// Get boot time for the current event we are checking.
		bootTime, err := parsing.GetEventBootTime(event)
		// If error with this event's boot-time, move on to the next event.
		if err != nil || bootTime <= 0 {
			level.Debug(t.logger).Log(xlog.ErrorKey(), "event has invalid boot-time", "event UUID", event.TransactionUUID)
			continue
		}

		// boot-time is greater than the event from caduceus.
		// This is an error, as it means this event is not the latest.
		if bootTime > incomingBootTime {
			t.logErrWithEventDetails(err, incomingWRP, event)
			return client.Event{}, errNewerBootTime
		}

		// Event has the same boot-time and destination as incoming event
		if bootTime == incomingBootTime && t.endValidator.ValidateType(event.Dest) && event.TransactionUUID != incomingWRP.TransactionUUID {
			// if duplicate events are not allowed, we should no longer continue doing calculations on the
			// event we got from caduceus. Throw an error.
			if t.discardDuplicate {
				t.logErrWithEventDetails(err, incomingWRP, event)
				return client.Event{}, errDuplicateFound
			}
		}

		// This event has a boot-time and destination we are searching for
		if bootTime == bootTimeSearch && t.initialValidator.ValidateType(event.Dest) {
			// See if we have found a matching event previously. If we haven't, save
			// the current event as the latestEvent.
			if !t.initialValidator.ValidateType(latestEvent.Dest) {
				latestEvent = event
			} else {
				// If we have found a matching event previously but this event
				// has an older birthdate, save this event instead.
				if event.BirthDate < latestEvent.BirthDate {
					latestEvent = event
				}
			}
		}
	}

	// if we have not found any events, return an error
	if len(latestEvent.TransactionUUID) == 0 {
		level.Error(t.logger).Log(xlog.MessageKey(), "no initial events found from codex", "incoming WRP", incomingWRP.TransactionUUID)
		return latestEvent, errNoInitialEvent
	}

	// return the event found
	return latestEvent, nil
}

func (t *TimeElapsedParser) logErrWithEventDetails(err error, incomingWRP wrp.Message, codexEvent client.Event) {
	deviceID, _ := parsing.GetDeviceID(eventRegex, incomingWRP.Destination)
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
