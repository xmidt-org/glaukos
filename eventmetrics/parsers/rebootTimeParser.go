package parsers

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/validation"
	"go.uber.org/zap"
)

const (
	firmwareLabel        = "firmware"
	hardwareLabel        = "hardware"
	rebootReasonLabel    = "reboot_reason"
	validationErrReason  = "validation_error"
	fatalErrReason       = "incoming_event_fatal_error"
	calculationErrReason = "time_elapsed_calculation_error"
	noHwFwReason         = "no_firmware_or_hardware_key"
	unknownReason        = "unknown"

	hardwareMetadataKey     = "/hw-model"
	firmwareMetadataKey     = "/fw-name"
	rebootReasonMetadataKey = "/hw-last-reboot-reason"
	invalidIncomingMsg      = "invalid incoming event"
)

var (
	errInvalidTimeElapsed = errors.New("invalid time elapsed calculated")
)

// CycleValidator validates a list of events.
type CycleValidator interface {
	Valid(events []interpreter.Event) (bool, error)
}

// EventsParser parses the relevant events from a device's history of events and returns those events.
type EventsParser interface {
	Parse(eventsHistory []interpreter.Event, currentEvent interpreter.Event) ([]interpreter.Event, error)
}

// EventClient is an interface that provides a list of events related to a device.
type EventClient interface {
	GetEvents(deviceID string) []interpreter.Event
}

// Finder returns a specific event in a list of events.
type Finder interface {
	Find(events []interpreter.Event, incomingEvent interpreter.Event) (interpreter.Event, error)
}

// RebootDurationParser is triggered whenever glaukos receives a fully-manageable event. Validates the event
// and the last boot-cycle, calculating the boot and reboot durations.
type RebootDurationParser struct {
	name                string
	finder              Finder
	eventValidator      validation.Validator
	entireCycleParser   EventsParser
	lastCycleParser     EventsParser
	rebootEventsParser  EventsParser
	lastCycleValidators []CycleValidator
	rebootValidators    []CycleValidator
	logger              *zap.Logger
	client              EventClient
	measures            Measures
}

// Name implements the Parser interface.
func (p *RebootDurationParser) Name() string {
	return p.name
}

// Parse takes an event, validates it, and calculates the time elapsed if everything is valid.
/*
	Steps:
	1. HW & FW: Get the hardware and firmware values stored in the event's metadata to use as labels in Prometheus metrics.
	2. Destination check: check that the incoming event is a fully-manageable evenp.
	2. Basic checks: Check that the boot-time and device id exists.
	3. Get events: Get history of events from codex, parse into slice with relevant events.
	4. Parse events: Parse into slice with last cycle's events.
	5. Validate events: run validation on each of the last cycle's events.
	6. Cycle validation: Validate the entire cycle
	7. Calculate time elapsed: If everything is valid, calculate the boot and reboot durations.
*/
func (p *RebootDurationParser) Parse(event interpreter.Event) {
	// get hardware and firmware from metadata to use in metrics as labels
	hardwareVal, firmwareVal, found := getHardwareFirmware(event)
	if !found {
		p.measures.RebootUnparsableCount.With(prometheus.Labels{hardwareLabel: hardwareVal,
			firmwareLabel: firmwareVal, reasonLabel: noHwFwReason}).Add(1.0)
	}

	// Make sure event is actually a fully-manageable event. Make sure event follows event regex and is a fully-mangeable event.
	eventType, err := event.EventType()
	if err != nil {
		p.measures.TotalUnparsableEvents.With(prometheus.Labels{parserLabel: p.name}).Add(1.0)
		p.measures.RebootUnparsableCount.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: fatalErrReason}).Add(1.0)
		p.logger.Error(invalidIncomingMsg, zap.Error(err), zap.String("event destination", event.Destination))
		return
	} else if eventType != "fully-manageable" {
		p.logger.Debug("wrong destination", zap.Error(err), zap.String("event destination", event.Destination))
		return
	}

	// Check that event passes necessary checks. If it doesn't it is impossible to continue and we should exit.
	if !p.basicChecks(event) {
		p.measures.TotalUnparsableEvents.With(prometheus.Labels{parserLabel: p.name}).Add(1.0)
		p.measures.RebootUnparsableCount.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: fatalErrReason}).Add(1.0)
		return
	}

	// Get the history of events and parse events relevant to the latest boot-cycle, into a slice.
	bootCycle, err := p.getEvents(event)
	if err != nil {
		p.measures.TotalUnparsableEvents.With(prometheus.Labels{parserLabel: p.name}).Add(1.0)
		p.measures.RebootUnparsableCount.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: fatalErrReason}).Add(1.0)
		return
	}

	// parse the events that need validation
	eventsToValidate, err := p.lastCycleParser.Parse(bootCycle, event)
	if err != nil {
		p.measures.TotalUnparsableEvents.With(prometheus.Labels{parserLabel: p.name}).Add(1.0)
		p.measures.RebootUnparsableCount.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: fatalErrReason}).Add(1.0)
		return
	}

	// validate events individually and as a cycle
	if !p.validateEvents(eventsToValidate, event) || !p.validateCycle(eventsToValidate) {
		p.measures.TotalUnparsableEvents.With(prometheus.Labels{parserLabel: p.name}).Add(1.0)
		p.measures.RebootUnparsableCount.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: validationErrReason}).Add(1.0)
		return
	}

	// TODO: validate reboot events

	// If there are calculation errors, add to the appropriate error counters.
	if valid := p.calculateDurations(bootCycle, event); !valid {
		p.measures.TotalUnparsableEvents.With(prometheus.Labels{parserLabel: p.name}).Add(1.0)
		p.measures.RebootUnparsableCount.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: calculationErrReason}).Add(1.0)
	}
}

// check that event has a boot-time and device id
func (p *RebootDurationParser) basicChecks(event interpreter.Event) bool {
	if bootTime, err := event.BootTime(); err != nil || bootTime <= 0 {
		p.logger.Error(invalidIncomingMsg, zap.Error(err))
		return false
	}

	_, err := event.DeviceID()
	if err != nil {
		p.logger.Error(invalidIncomingMsg, zap.Error(err))
		return false
	}

	return true
}

// get history of events and return relevant events
func (p *RebootDurationParser) getEvents(event interpreter.Event) ([]interpreter.Event, error) {
	deviceID, err := event.DeviceID()
	if err != nil {
		p.logger.Error("error getting device id", zap.Error(err))
		return []interpreter.Event{}, err
	}

	events := p.client.GetEvents(deviceID)
	bootCycle, err := p.entireCycleParser.Parse(events, event)
	if err != nil {
		p.logger.Info("parsing error", zap.Error(err), zap.String("event id", event.TransactionUUID), zap.String("device id", deviceID))
		return []interpreter.Event{}, err
	}

	// make sure slice is sorted oldest to newest
	sort.Slice(bootCycle, func(a, b int) bool {
		boottimeA, _ := bootCycle[a].BootTime()
		boottimeB, _ := bootCycle[b].BootTime()
		if boottimeA != boottimeB {
			return boottimeA < boottimeB
		}
		return bootCycle[a].Birthdate < bootCycle[b].Birthdate
	})

	return bootCycle, nil

}

// find individual errors with events and add to metrics
func (p *RebootDurationParser) validateEvents(events []interpreter.Event, event interpreter.Event) bool {
	deviceID, _ := event.DeviceID()
	allValid := true
	for _, event := range events {
		if valid, err := p.eventValidator.Valid(event); !valid {
			allValid = false
			p.logEventError(err, event, deviceID)
		}
	}

	return allValid
}

// run all cycle validators on the list of events, add tags to metrics
func (p *RebootDurationParser) validateCycle(cycle []interpreter.Event) bool {
	allValid := true
	var allErrors validation.Errors
	for _, cycleValidator := range p.lastCycleValidators {
		if valid, err := cycleValidator.Valid(cycle); !valid {
			allValid = false
			allErrors = append(allErrors, err)
		}
	}

	if allErrors != nil {
		p.logger.Info("invalid cycle", zap.String("tags", tagsToString(allErrors.UniqueTags())))
	}

	for _, tag := range allErrors.UniqueTags() {
		p.measures.RebootCycleErrors.With(prometheus.Labels{reasonLabel: tag.String()}).Add(1.0)
	}

	return allValid
}

// Calculates difference between birthdate and boot-time of the event, as well as the
// duration between reboot-pending event and fully-manageable event. Logs in metrics and returns false
// if there is an error, true if no errors.
func (p *RebootDurationParser) calculateDurations(cycle []interpreter.Event, event interpreter.Event) bool {
	allValid := true
	labels := getTimeElapsedHistogramLabels(event)
	if bootDuration, err := p.calculateBootDuration(event); err != nil {
		allValid = false
	} else {
		p.measures.BootToManageableHistogram.With(labels).Observe(bootDuration)
	}

	if rebootDuration, err := p.timeBetweenEvents(cycle, event); err != nil {
		if !errors.Is(err, errInvalidTimeElapsed) {
			allValid = false
		}
	} else {
		p.measures.RebootToManageableHistogram.With(labels).Observe(rebootDuration)
	}

	return allValid
}

// calcualate the time between getting new boot-time and birthdate of event
func (p *RebootDurationParser) calculateBootDuration(event interpreter.Event) (float64, error) {
	bootTime, _ := event.BootTime()
	bootTimeUnix := time.Unix(bootTime, 0)
	birthdateUnix := time.Unix(0, event.Birthdate)
	var bootDuration float64
	if bootTime > 0 && event.Birthdate > 0 {
		bootDuration = birthdateUnix.Sub(bootTimeUnix).Seconds()
	}

	if bootDuration <= 0 {
		deviceID, _ := event.DeviceID()
		p.logger.Error("invalid time calculated", zap.String("deviceID", deviceID), zap.Float64("invalid time elapsed", bootDuration), zap.String("incoming event", event.TransactionUUID))
		return -1, errInvalidTimeElapsed
	}

	return bootDuration, nil
}

// calculate the time between current event and another event in the history
func (p *RebootDurationParser) timeBetweenEvents(bootCycle []interpreter.Event, event interpreter.Event) (float64, error) {
	currentBirthdate := time.Unix(0, event.Birthdate)
	startingEvent, err := p.finder.Find(bootCycle, event)
	if err != nil {
		p.logger.Error("time calculation error", zap.Error(err))
		return -1, err
	}

	startingEventTime := time.Unix(0, startingEvent.Birthdate)
	var timeElapsed float64
	if event.Birthdate > 0 && startingEvent.Birthdate > 0 {
		timeElapsed = currentBirthdate.Sub(startingEventTime).Seconds()
	}

	if timeElapsed <= 0 {
		deviceID, _ := event.DeviceID()
		p.logger.Error("time calculation error",
			zap.String("deviceID", deviceID),
			zap.String("incoming event", event.TransactionUUID),
			zap.String("comparison event", startingEvent.TransactionUUID),
			zap.Float64("time calculated", timeElapsed))
		return -1, errInvalidTimeElapsed
	}

	return timeElapsed, nil

}

// log an event error to metrics
func (p *RebootDurationParser) logEventError(err error, event interpreter.Event, deviceID string) {
	hardwareVal, firmwareVal, _ := getHardwareFirmware(event)
	eventID := event.TransactionUUID

	var taggedErrs validation.TaggedErrors
	var taggedErr validation.TaggedError
	if errors.As(err, &taggedErrs) {
		p.logger.Info("event validation error", zap.String("tags", tagsToString(taggedErrs.UniqueTags())), zap.String("event id", eventID), zap.String("device id", deviceID))
		for _, tag := range taggedErrs.UniqueTags() {
			p.measures.RebootEventErrors.With(prometheus.Labels{firmwareLabel: firmwareVal,
				hardwareLabel: hardwareVal, reasonLabel: tag.String()}).Add(1.0)
		}
	} else if errors.As(err, &taggedErr) {
		p.logger.Info("event validation error", zap.String("tags", taggedErr.Tag().String()), zap.String("event id", eventID), zap.String("device id", deviceID))
		p.measures.RebootEventErrors.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: taggedErr.Tag().String()}).Add(1.0)
	} else if err != nil {
		p.logger.Info("event validation error; no tags", zap.Error(err), zap.String("event id", eventID), zap.String("device id", deviceID))
		p.measures.RebootEventErrors.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: validation.Unknown.String()}).Add(1.0)
	}
}

// grab relevant information from event metadata and return prometheus labels
func getTimeElapsedHistogramLabels(event interpreter.Event) prometheus.Labels {
	hardwareVal, firmwareVal, _ := getHardwareFirmware(event)
	rebootReason, reasonFound := event.GetMetadataValue(rebootReasonMetadataKey)
	if !reasonFound {
		rebootReason = unknownReason
	}

	return prometheus.Labels{
		hardwareLabel:     hardwareVal,
		firmwareLabel:     firmwareVal,
		rebootReasonLabel: rebootReason,
	}
}

// get hardware and firmware values from event metadata, returning false if either one or both are not found
func getHardwareFirmware(event interpreter.Event) (hardwareVal string, firmwareVal string, found bool) {
	hardwareVal, hardwareFound := event.GetMetadataValue(hardwareMetadataKey)
	firmwareVal, firmwareFound := event.GetMetadataValue(firmwareMetadataKey)

	found = true
	if !hardwareFound || !firmwareFound {
		found = false
		if !hardwareFound {
			hardwareVal = unknownReason
		}

		if !firmwareFound {
			firmwareVal = unknownReason
		}
	}

	return
}

func tagsToString(tags []validation.Tag) string {
	var output strings.Builder
	output.WriteRune('[')
	for i, tag := range tags {
		if i > 0 {
			output.WriteRune(',')
			output.WriteRune(' ')
		}
		output.WriteString(tag.String())
	}
	output.WriteRune(']')
	return output.String()
}
