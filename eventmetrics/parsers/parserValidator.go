package parsers

import (
	"errors"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/interpreter/validation"
	"go.uber.org/zap"
)

var (
	errFatal      = errors.New("fatal error")
	errValidation = errors.New("validation error")
)

// EventsParser parses the relevant events from a device's history of events and returns those events.
type EventsParser interface {
	Parse(eventsHistory []interpreter.Event, currentEvent interpreter.Event) ([]interpreter.Event, error)
}

func defaultEventsValidationCallback(_ interpreter.Event, _ bool, _ error) { return }
func defaultCycleValidationCallback(_ bool, _ error)                       { return }
func defaultActivateFunc(_ []interpreter.Event, _ interpreter.Event) bool  { return true }

type parserValidator struct {
	cycleParser              EventsParser
	cycleValidator           history.CycleValidator
	eventsValidator          validation.Validator
	shouldActivate           func([]interpreter.Event, interpreter.Event) bool
	eventsValidationCallback func(interpreter.Event, bool, error)
	cycleValidationCallback  func(bool, error)
}

func (p *parserValidator) Validate(events []interpreter.Event, currentEvent interpreter.Event) (bool, error) {
	p.setDefaults()
	if !p.shouldActivate(events, currentEvent) {
		return true, nil
	}

	allValid := true
	cycle, err := p.cycleParser.Parse(events, currentEvent)
	if err != nil {
		return false, errFatal
	}

	for _, event := range cycle {
		valid, eventErr := p.eventsValidator.Valid(event)
		if !valid {
			allValid = false
		}
		p.eventsValidationCallback(event, valid, eventErr)
	}

	cycleValid, cycleErr := p.cycleValidator.Valid(cycle)
	if !cycleValid {
		allValid = false
	}
	p.cycleValidationCallback(cycleValid, cycleErr)

	if !allValid {
		return false, errValidation
	}

	return true, nil
}

func (p *parserValidator) setDefaults() {
	if p.shouldActivate == nil {
		p.shouldActivate = defaultActivateFunc
	}

	if p.cycleParser == nil {
		p.cycleParser = history.DefaultCycleParser(nil)
	}

	if p.eventsValidator == nil {
		p.eventsValidator = validation.DefaultValidator()
	}

	if p.cycleValidator == nil {
		p.cycleValidator = history.DefaultCycleValidator()
	}

	if p.eventsValidationCallback == nil {
		p.eventsValidationCallback = defaultEventsValidationCallback
	}

	if p.cycleValidationCallback == nil {
		p.cycleValidationCallback = defaultCycleValidationCallback
	}
}

// log an cycle error to metrics
func logCycleErr(err error, counter *prometheus.CounterVec, logger *zap.Logger) {
	var taggedErrs validation.TaggedErrors
	var taggedErr validation.TaggedError
	if errors.As(err, &taggedErrs) {
		logger.Info("invalid cycle", zap.String("tags", tagsToString(taggedErrs.UniqueTags())))
		for _, tag := range taggedErrs.UniqueTags() {
			counter.With(prometheus.Labels{reasonLabel: tag.String()}).Add(1.0)
		}
	} else if errors.As(err, &taggedErr) {
		logger.Info("invalid cycle", zap.String("tags", taggedErr.Tag().String()))
		counter.With(prometheus.Labels{reasonLabel: taggedErr.Tag().String()}).Add(1.0)
	} else if err != nil {
		logger.Info("invalid cycle; no tags", zap.Error(err))
		counter.With(prometheus.Labels{reasonLabel: validation.Unknown.String()}).Add(1.0)
	}
}

// log an event error to metrics
func logEventError(logger *zap.Logger, counter *prometheus.CounterVec, err error, event interpreter.Event) {
	hardwareVal, firmwareVal, _ := getHardwareFirmware(event)
	deviceID, _ := event.DeviceID()
	eventID := event.TransactionUUID

	var taggedErrs validation.TaggedErrors
	var taggedErr validation.TaggedError
	if errors.As(err, &taggedErrs) {
		logger.Info("event validation error", zap.String("tags", tagsToString(taggedErrs.UniqueTags())), zap.String("event id", eventID), zap.String("device id", deviceID))
		for _, tag := range taggedErrs.UniqueTags() {
			counter.With(prometheus.Labels{firmwareLabel: firmwareVal,
				hardwareLabel: hardwareVal, reasonLabel: tag.String()}).Add(1.0)
		}
	} else if errors.As(err, &taggedErr) {
		logger.Info("event validation error", zap.String("tags", taggedErr.Tag().String()), zap.String("event id", eventID), zap.String("device id", deviceID))
		counter.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: taggedErr.Tag().String()}).Add(1.0)
	} else if err != nil {
		logger.Info("event validation error; no tags", zap.Error(err), zap.String("event id", eventID), zap.String("device id", deviceID))
		counter.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: validation.Unknown.String()}).Add(1.0)
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
