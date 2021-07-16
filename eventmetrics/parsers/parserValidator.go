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
		p.shouldActivate = func(_ []interpreter.Event, _ interpreter.Event) bool { return true }
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
		p.eventsValidationCallback = func(_ interpreter.Event, _ bool, _ error) {
			// default empty function
		}
	}

	if p.cycleValidationCallback == nil {
		p.cycleValidationCallback = func(_ bool, _ error) {
			// default empty function
		}
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
		logger.Info("event validation error", zap.String("tags", tagsToString(taggedErrs.UniqueTags())), zap.String(eventIDKey, eventID), zap.String(deviceIDKey, deviceID))
		for _, tag := range taggedErrs.UniqueTags() {
			counter.With(prometheus.Labels{firmwareLabel: firmwareVal,
				hardwareLabel: hardwareVal, reasonLabel: tag.String()}).Add(1.0)
		}
	} else if errors.As(err, &taggedErr) {
		logger.Info("event validation error", zap.String("tags", taggedErr.Tag().String()), zap.String(eventIDKey, eventID), zap.String(deviceIDKey, deviceID))
		counter.With(prometheus.Labels{firmwareLabel: firmwareVal,
			hardwareLabel: hardwareVal, reasonLabel: taggedErr.Tag().String()}).Add(1.0)
	} else if err != nil {
		logger.Info("event validation error; no tags", zap.Error(err), zap.String(eventIDKey, eventID), zap.String(deviceIDKey, deviceID))
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
