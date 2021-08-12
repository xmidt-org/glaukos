/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package parsers

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/interpreter/validation"
	"go.uber.org/zap"
)

const (
	unknownLabelValue = "unknown"
)

var (
	errFatal          = errors.New("fatal error")
	errValidation     = errors.New("validation error")
	errNonExistentKey = errors.New("key does not exist")
)

// EventsParser parses the relevant events from a device's history of events and returns those events.
type EventsParser interface {
	Parse(eventsHistory []interpreter.Event, currentEvent interpreter.Event) ([]interpreter.Event, error)
}

type cycleValidation struct {
	parser    EventsParser
	validator history.CycleValidator
	callback  func(currentEvent interpreter.Event, valid bool, err error)
}

type eventValidation struct {
	validator validation.Validator
	callback  func(event interpreter.Event, valid bool, err error)
}

type parserValidator struct {
	cycleParser             EventsParser
	cycleValidator          history.CycleValidator
	cycleValidationCallback func(currentEvent interpreter.Event, valid bool, err error)
	eventValidator          validation.Validator
	eventValidationCallback func(event interpreter.Event, valid bool, err error)
	shouldActivate          func(events []interpreter.Event, currentEvent interpreter.Event) bool
}

// NewParserValidator creates a new parserValidator.
func NewParserValidator(cycleValidator cycleValidation, eventValidator eventValidation, activateFunc func([]interpreter.Event, interpreter.Event) bool) *parserValidator {
	if activateFunc == nil {
		activateFunc = func(_ []interpreter.Event, _ interpreter.Event) bool { return true }
	}

	if cycleValidator.parser == nil {
		cycleValidator.parser = history.DefaultCycleParser(nil)
	}

	if cycleValidator.validator == nil {
		cycleValidator.validator = history.DefaultCycleValidator()
	}

	if cycleValidator.callback == nil {
		cycleValidator.callback = func(_ interpreter.Event, _ bool, _ error) {
			// default empty function
		}
	}

	if eventValidator.validator == nil {
		eventValidator.validator = validation.DefaultValidator()
	}

	if eventValidator.callback == nil {
		eventValidator.callback = func(_ interpreter.Event, _ bool, _ error) {
			// default empty function
		}
	}

	return &parserValidator{
		cycleParser:             cycleValidator.parser,
		cycleValidator:          cycleValidator.validator,
		cycleValidationCallback: cycleValidator.callback,
		eventValidator:          eventValidator.validator,
		eventValidationCallback: eventValidator.callback,
		shouldActivate:          activateFunc,
	}

}

func (p *parserValidator) Validate(events []interpreter.Event, currentEvent interpreter.Event) (bool, error) {
	if !p.shouldActivate(events, currentEvent) {
		return true, nil
	}

	allValid := true
	cycle, err := p.cycleParser.Parse(events, currentEvent)
	if err != nil {
		return false, errFatal
	}

	for _, event := range cycle {
		valid, eventErr := p.eventValidator.Valid(event)
		if !valid {
			allValid = false
		}
		p.eventValidationCallback(event, valid, eventErr)
	}

	cycleValid, cycleErr := p.cycleValidator.Valid(cycle)
	if !cycleValid {
		allValid = false
	}
	p.cycleValidationCallback(currentEvent, cycleValid, cycleErr)

	if !allValid {
		return false, errValidation
	}

	return true, nil
}

// log an cycle error to metrics
func logCycleErr(currentEvent interpreter.Event, err error, counter *prometheus.CounterVec, logger *zap.Logger) {
	const (
		deviceIDKey = "device id"
	)

	deviceID, _ := currentEvent.DeviceID()
	var taggedErrs validation.TaggedErrors
	var taggedErr validation.TaggedError
	if errors.As(err, &taggedErrs) {
		logger.Info("invalid cycle", zap.String(deviceIDKey, deviceID), zap.Strings("tags", validation.TagsToStrings(taggedErrs.UniqueTags())))
		if counter != nil {
			for _, tag := range taggedErrs.UniqueTags() {
				counter.With(prometheus.Labels{reasonLabel: tag.String()}).Add(1.0)
			}
		}
	} else if errors.As(err, &taggedErr) {
		logger.Info("invalid cycle", zap.String(deviceIDKey, deviceID), zap.String("tags", taggedErr.Tag().String()))
		if counter != nil {
			counter.With(prometheus.Labels{reasonLabel: taggedErr.Tag().String()}).Add(1.0)
		}

	} else if err != nil {
		logger.Info("invalid cycle; no tags", zap.String(deviceIDKey, deviceID), zap.Error(err))
		if counter != nil {
			counter.With(prometheus.Labels{reasonLabel: validation.Unknown.String()}).Add(1.0)
		}
	}
}

// log an event error to metrics
func logEventError(logger *zap.Logger, counter *prometheus.CounterVec, err error, event interpreter.Event) {
	const (
		eventIDKey  = "event id"
		deviceIDKey = "device id"
	)

	hardwareVal, firmwareVal, _ := getHardwareFirmware(event)
	deviceID, _ := event.DeviceID()
	eventID := event.TransactionUUID

	var taggedErrs validation.TaggedErrors
	var taggedErr validation.TaggedError
	if errors.As(err, &taggedErrs) {
		logger.Info("event validation error", zap.Strings("tags", validation.TagsToStrings(taggedErrs.UniqueTags())), zap.String(eventIDKey, eventID), zap.String(deviceIDKey, deviceID))
		if counter != nil {
			for _, tag := range taggedErrs.UniqueTags() {
				counter.With(prometheus.Labels{firmwareLabel: firmwareVal,
					hardwareLabel: hardwareVal, reasonLabel: tag.String()}).Add(1.0)
			}
		}
	} else if errors.As(err, &taggedErr) {
		logger.Info("event validation error", zap.String("tags", taggedErr.Tag().String()), zap.String(eventIDKey, eventID), zap.String(deviceIDKey, deviceID))
		if counter != nil {
			counter.With(prometheus.Labels{firmwareLabel: firmwareVal,
				hardwareLabel: hardwareVal, reasonLabel: taggedErr.Tag().String()}).Add(1.0)
		}
	} else if err != nil {
		logger.Info("event validation error; no tags", zap.Error(err), zap.String(eventIDKey, eventID), zap.String(deviceIDKey, deviceID))
		if counter != nil {
			counter.With(prometheus.Labels{firmwareLabel: firmwareVal,
				hardwareLabel: hardwareVal, reasonLabel: validation.Unknown.String()}).Add(1.0)
		}
	}
}

// get hardware and firmware values from event metadata, returning false if either one or both are not found
func getHardwareFirmware(event interpreter.Event) (hardwareVal string, firmwareVal string, found bool) {
	hardwareVal, hardwareFound := event.GetMetadataValue(hardwareMetadataKey)
	firmwareVal, firmwareFound := event.GetMetadataValue(firmwareMetadataKey)

	found = true
	if !hardwareFound {
		hardwareVal = unknownLabelValue
		found = false
	}
	if !firmwareFound {
		firmwareVal = unknownLabelValue
		found = false
	}

	return
}

// grab relevant information from event metadata and return prometheus labels
func getTimeElapsedHistogramLabels(event interpreter.Event) prometheus.Labels {
	hardwareVal, firmwareVal, _ := getHardwareFirmware(event)
	rebootReason, reasonFound := event.GetMetadataValue(rebootReasonMetadataKey)
	if !reasonFound {
		rebootReason = unknownLabelValue
	}

	return prometheus.Labels{
		hardwareLabel:     hardwareVal,
		firmwareLabel:     firmwareVal,
		rebootReasonLabel: rebootReason,
	}
}
