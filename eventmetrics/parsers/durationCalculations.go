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
	"time"

	"github.com/xmidt-org/interpreter"
	"go.uber.org/zap"
)

var (
	errCalculation   = errors.New("time elapsed calculation error")
	errEventNotFound = errors.New("event not found")

	errMissingFinder = errors.New("missing Finder")
)

// CalculatorFunc is a function that calculates a duration and returns an error if there is a
// problem while performing the calculations.
type CalculatorFunc func([]interpreter.Event, interpreter.Event) error

// Calculate implements the DurationCalculator interface.
func (cf CalculatorFunc) Calculate(events []interpreter.Event, event interpreter.Event) error {
	return cf(events, event)
}

// BootDurationCalculator returns a CalculatorFunc that calculates the time between the birthdate and the boot-time
// of an event, calling the successCallback if a duration is successfully calculated.
func BootDurationCalculator(logger *zap.Logger, successCallback func(interpreter.Event, float64)) CalculatorFunc {
	return func(events []interpreter.Event, event interpreter.Event) error {
		bootTime, _ := event.BootTime()
		bootTimeUnix := time.Unix(bootTime, 0)
		birthdateUnix := time.Unix(0, event.Birthdate)
		var bootDuration float64
		if bootTime > 0 && event.Birthdate > 0 {
			bootDuration = birthdateUnix.Sub(bootTimeUnix).Seconds()
		}

		if bootDuration <= 0 {
			deviceID, _ := event.DeviceID()
			logger.Error("invalid time calculated", zap.String("deviceID", deviceID), zap.Float64("invalid time elapsed", bootDuration), zap.String("incoming event", event.TransactionUUID))
			return errCalculation
		}

		if successCallback != nil {
			successCallback(event, bootDuration)
		}

		return nil
	}
}

// EventToCurrentCalculator calculates the difference between the current event and a previous event.
type EventToCurrentCalculator struct {
	eventFinder     Finder
	successCallback func(currentEvent interpreter.Event, foundEvent interpreter.Event, duration float64)
	logger          *zap.Logger
}

// NewEventToCurrentCalculator creates a new EventToCurrentCalculator and an error if the finder is nil.
func NewEventToCurrentCalculator(eventFinder Finder, successCallback func(currentEvent interpreter.Event, foundEvent interpreter.Event, duration float64), logger *zap.Logger) (*EventToCurrentCalculator, error) {
	if eventFinder == nil {
		return nil, errMissingFinder
	}

	if successCallback == nil {
		successCallback = func(_ interpreter.Event, _ interpreter.Event, _ float64) {
			// default empty function
		}
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &EventToCurrentCalculator{
		eventFinder:     eventFinder,
		successCallback: successCallback,
		logger:          logger,
	}, nil
}

// Calculate implements the DurationCalculator interface by subtracting the birthdates of the two events.
func (c *EventToCurrentCalculator) Calculate(events []interpreter.Event, event interpreter.Event) error {
	if c.logger == nil {
		c.logger = zap.NewNop()
	}

	if c.eventFinder == nil {
		return errMissingFinder
	}

	currentBirthdate := time.Unix(0, event.Birthdate)
	startingEvent, err := c.eventFinder.Find(events, event)
	if err != nil {
		c.logger.Error("time calculation error", zap.Error(err))
		return errEventNotFound
	}

	startingEventTime := time.Unix(0, startingEvent.Birthdate)
	var timeElapsed float64
	if event.Birthdate > 0 && startingEvent.Birthdate > 0 {
		timeElapsed = currentBirthdate.Sub(startingEventTime).Seconds()
	}

	if timeElapsed <= 0 {
		deviceID, _ := event.DeviceID()
		c.logger.Error("time calculation error",
			zap.String("deviceID", deviceID),
			zap.String("incoming event", event.TransactionUUID),
			zap.String("comparison event", startingEvent.TransactionUUID),
			zap.Float64("time calculated", timeElapsed))
		return errCalculation
	}

	if c.successCallback != nil {
		c.successCallback(event, startingEvent, timeElapsed)
	}

	return nil
}
