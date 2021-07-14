package parsers

import (
	"errors"
	"sort"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/interpreter"
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

// DurationCalculator calculates the different durations in a boot cycle.
type DurationCalculator interface {
	Calculate([]interpreter.Event, interpreter.Event) error
}

// ParserValidator parses (if needed) the events from the list of events passed in and runs validation on this
// subset of events.
type ParserValidator interface {
	Validate(events []interpreter.Event, currentEvent interpreter.Event) (bool, error)
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
	name                 string
	relevantEventsParser EventsParser
	parserValidators     []ParserValidator
	calculators          []DurationCalculator
	logger               *zap.Logger
	client               EventClient
	measures             Measures
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
	3. Basic checks: Check that the boot-time and device id exists.
	4. Get events: Get history of events from codex, parse into slice with relevant events.
	5. Parse and Validate: Go through parsers and parse and validate as needed.
	6. Calculate time elapsed: Go through duration calculators to calculate durations and add to appropriate histograms.
*/
func (p *RebootDurationParser) Parse(currentEvent interpreter.Event) {
	// get hardware and firmware from metadata to use in metrics as labels
	hardwareVal, firmwareVal, found := getHardwareFirmware(currentEvent)
	if !found {
		p.measures.RebootUnparsableCount.With(prometheus.Labels{hardwareLabel: hardwareVal,
			firmwareLabel: firmwareVal, reasonLabel: noHwFwReason}).Add(1.0)
	}

	// Make sure event is actually a fully-manageable event. Make sure event follows event regex and is a fully-mangeable event.
	eventType, err := currentEvent.EventType()
	if err != nil {
		p.addToUnparsableCounters(firmwareVal, hardwareVal, fatalErrReason)
		p.logger.Error(invalidIncomingMsg, zap.Error(err), zap.String("event destination", currentEvent.Destination))
		return
	} else if eventType != "fully-manageable" {
		p.logger.Debug("wrong destination", zap.Error(err), zap.String("event destination", currentEvent.Destination))
		return
	}

	// Check that event passes necessary checks. If it doesn't it is impossible to continue and we should exit.
	if !p.basicChecks(currentEvent) {
		p.addToUnparsableCounters(firmwareVal, hardwareVal, fatalErrReason)
		return
	}

	// Get the history of events and parse events relevant to the latest boot-cycle, into a slice.
	relevantEvents, err := p.getEvents(currentEvent)
	if err != nil {
		p.addToUnparsableCounters(firmwareVal, hardwareVal, fatalErrReason)
		return
	}

	allValid := true
	for _, parserValidator := range p.parserValidators {
		if valid, _ := parserValidator.Validate(relevantEvents, currentEvent); !valid {
			allValid = false
		}
	}

	if !allValid {
		p.addToUnparsableCounters(firmwareVal, hardwareVal, validationErrReason)
		return
	}

	calculationValid := true
	for _, calculator := range p.calculators {
		if err := calculator.Calculate(relevantEvents, currentEvent); err != nil {
			// no need to log in metrics if event doesn't exist
			if !errors.Is(err, errEventNotFound) {
				calculationValid = false
			}
		}
	}

	if !calculationValid {
		p.addToUnparsableCounters(firmwareVal, hardwareVal, calculationErrReason)
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
func (p *RebootDurationParser) getEvents(currentEvent interpreter.Event) ([]interpreter.Event, error) {
	deviceID, err := currentEvent.DeviceID()
	if err != nil {
		p.logger.Error("error getting device id", zap.Error(err))
		return []interpreter.Event{}, err
	}

	events := p.client.GetEvents(deviceID)
	bootCycle, err := p.relevantEventsParser.Parse(events, currentEvent)
	if err != nil {
		p.logger.Info("parsing error", zap.Error(err), zap.String("event id", currentEvent.TransactionUUID), zap.String("device id", deviceID))
		return []interpreter.Event{}, err
	}

	// make sure slice is sorted newest to oldest
	sort.Slice(bootCycle, func(a, b int) bool {
		boottimeA, _ := bootCycle[a].BootTime()
		boottimeB, _ := bootCycle[b].BootTime()
		if boottimeA != boottimeB {
			return boottimeA > boottimeB
		}
		return bootCycle[a].Birthdate > bootCycle[b].Birthdate
	})

	return bootCycle, nil

}

func (p *RebootDurationParser) addToUnparsableCounters(firmwareVal string, hardwareVal string, reason string) {
	p.measures.TotalUnparsableEvents.With(prometheus.Labels{parserLabel: p.name}).Add(1.0)
	p.measures.RebootUnparsableCount.With(prometheus.Labels{firmwareLabel: firmwareVal,
		hardwareLabel: hardwareVal, reasonLabel: reason}).Add(1.0)
}
