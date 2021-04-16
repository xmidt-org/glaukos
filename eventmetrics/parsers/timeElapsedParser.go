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
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/interpreter/validation"

	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"go.uber.org/zap"
)

const (
	firmwareLabel               = "firmware"
	hardwareLabel               = "hardware"
	rebootReasonLabel           = "reboot_reason"
	noFwHwReason                = "no_firmware_or_hardware_key"
	idParseReason               = "device_ID_unparseable"
	unknownReason               = "unknown"
	invalidTimeCalculatedReason = "invalid_time_calculated"

	hardwareMetadataKey     = "/hw-model"
	firmwareMetadataKey     = "/fw-name"
	rebootReasonMetadataKey = "/hw-last-reboot-reason"

	invalidIncomingMsg = "invalid incoming event"
)

// EventClient is an interface that provides a list of events related to a device.
type EventClient interface {
	GetEvents(deviceID string) []interpreter.Event
}

// Finder returns a specific event in a list of events.
type Finder interface {
	Find(events []interpreter.Event, incomingEvent interpreter.Event) (interpreter.Event, error)
}

// ErrorWithEvent is an optional interface that errors can implement if the error contains an event that is related
// to the error.
type ErrorWithEvent interface {
	Event() interpreter.Event
}

// TimeElapsedConfig holds the configuration for a time-elapsed parser.
type TimeElapsedConfig struct {
	Name            string
	SearchedSession string
	IncomingEvent   EventConfig
	SearchedEvent   EventConfig
}

// TimeElapsedParser is a parser that calculates the time between two events.
type TimeElapsedParser struct {
	finder        Finder
	incomingEvent EventInfo
	searchedEvent EventInfo
	name          string
	logger        *zap.Logger
	client        EventClient
	measures      Measures
}

// NewTimeElapsedParser creates a TimeElapsedParser from a TimeElapsedConfig or returns an error if there one cannot be created.
func NewTimeElapsedParser(config TimeElapsedConfig, client EventClient, logger *zap.Logger, measures Measures, currentTime TimeFunc) (*TimeElapsedParser, error) {
	incomingEvent, err := NewEventInfo(config.IncomingEvent, currentTime)
	if err != nil {
		return nil, err
	}

	var searchedEvent EventInfo
	if len(config.SearchedEvent.Regex) == 0 {
		searchedEvent = EventInfo{Regex: incomingEvent.Regex, CalculateUsing: Boottime}
		incomingEvent.CalculateUsing = Birthdate
	} else {
		searchedEvent, err = NewEventInfo(config.SearchedEvent, currentTime)
		if err != nil {
			return nil, err
		}
	}

	var finder Finder
	sessionType := ParseSessionType(config.SearchedSession)
	comparators := history.Comparators([]history.Comparator{
		history.OlderBootTimeComparator(),
		history.DuplicateEventComparator(incomingEvent.Regex),
	})

	if len(config.SearchedEvent.Regex) == 0 {
		finder = history.EventHistoryIterator(comparators)
	} else if sessionType == Current {
		finder = history.CurrentSessionFinder(searchedEvent.Validator, comparators)
	} else {
		finder = history.LastSessionFinder(searchedEvent.Validator, comparators)
	}

	return &TimeElapsedParser{
		finder:        finder,
		incomingEvent: incomingEvent,
		searchedEvent: searchedEvent,
		logger:        logger,
		client:        client,
		measures:      measures,
		name:          config.Name,
	}, nil
}

// Parse implements the Parser interface. Calculates the time-elapsed and then adds to prometheus metrics if possible.
func (t *TimeElapsedParser) Parse(event interpreter.Event) {
	restartTime, err := t.calculateTimeElapsed(event)
	if err != nil || restartTime <= 0 {
		var logError validation.MetricsLogError
		if errors.As(err, &logError) {
			t.measures.UnparsableEventsCount.With(prometheus.Labels{parserLabel: t.name, reasonLabel: logError.ErrorLabel()}).Add(1.0)
		} else {
			t.measures.UnparsableEventsCount.With(prometheus.Labels{parserLabel: t.name, reasonLabel: unknownReason}).Add(1.0)
		}
		return
	}

	hardwareVal, hardwareFound := event.GetMetadataValue(hardwareMetadataKey)
	firmwareVal, firmwareFound := event.GetMetadataValue(firmwareMetadataKey)
	rebootReason, reasonFound := event.GetMetadataValue(rebootReasonMetadataKey)

	if !hardwareFound || !firmwareFound {
		t.measures.UnparsableEventsCount.With(prometheus.Labels{parserLabel: t.name, reasonLabel: noFwHwReason}).Add(1.0)
		return
	}

	if !reasonFound {
		rebootReason = unknownReason
	}

	if histogram, ok := t.measures.TimeElapsedHistograms[t.name]; ok {
		labels := prometheus.Labels{
			hardwareLabel:     hardwareVal,
			firmwareLabel:     firmwareVal,
			rebootReasonLabel: rebootReason,
		}
		histogram.With(labels).Observe(restartTime)
	} else {
		t.logger.Error("No histogram found for this time elapsed parser")
	}
}

// Name returns the name of the parser. Implements the Parser interface.
func (t *TimeElapsedParser) Name() string {
	return t.name
}

// fixConfig takes in a TimeElapsedConfig and sets the names and default time validation for parser configs if needed.
func fixConfig(config TimeElapsedConfig, defaultValidFrom time.Duration) TimeElapsedConfig {
	name := strings.ReplaceAll(strings.TrimSpace(config.Name), " ", "_")
	config.Name = fmt.Sprintf("TEP_%s", name)
	if config.IncomingEvent.ValidFrom == 0 {
		config.IncomingEvent.ValidFrom = defaultValidFrom
	}

	if config.SearchedEvent.ValidFrom == 0 {
		config.SearchedEvent.ValidFrom = defaultValidFrom
	}

	return config
}

// calculateTimeElapsed is the function that does the main time-elapsed calculations.
func (t *TimeElapsedParser) calculateTimeElapsed(incomingEvent interpreter.Event) (float64, error) {
	if valid, err := t.incomingEvent.Valid(incomingEvent); !valid {
		if errors.Is(err, validation.ErrInvalidEventType) {
			t.logger.Info(invalidIncomingMsg, zap.Error(err), zap.String("event destination", incomingEvent.Destination))
		} else {
			t.logger.Error(invalidIncomingMsg, zap.Error(err), zap.String("event destination", incomingEvent.Destination))
		}
		return -1, err
	}

	deviceID, err := incomingEvent.DeviceID()
	if err != nil {
		t.logger.Error(invalidIncomingMsg, zap.Error(err))
		return -1, TimeElapsedCalculationErr{err: err, errLabel: idParseReason}
	}

	events := t.client.GetEvents(deviceID)
	oldEvent, err := t.finder.Find(events, incomingEvent)
	if err != nil {
		t.logErrWithEventDetails(invalidIncomingMsg, err, incomingEvent)
		return -1, err
	}

	oldEventTime := ParseTime(oldEvent, t.searchedEvent.CalculateUsing)
	newEventTime := ParseTime(incomingEvent, t.incomingEvent.CalculateUsing)
	var timeElapsed float64
	if !oldEventTime.IsZero() && !newEventTime.IsZero() {
		timeElapsed = newEventTime.Sub(oldEventTime).Seconds()
	}

	if timeElapsed <= 0 {
		err = TimeElapsedCalculationErr{timeElapsed: timeElapsed, oldEvent: oldEvent, errLabel: invalidTimeCalculatedReason}
		t.logErrWithEventDetails("invalid calculation", err, incomingEvent)
		return -1, err
	}

	return timeElapsed, nil
}

// logs errors with events' transaction uuids.
func (t *TimeElapsedParser) logErrWithEventDetails(msg string, err error, incomingEvent interpreter.Event) {
	deviceID, _ := incomingEvent.DeviceID()
	var eventErr ErrorWithEvent
	if errors.As(err, &eventErr) {
		if len(eventErr.Event().TransactionUUID) > 0 {
			t.logger.Error(msg, zap.Error(err), zap.String("deviceID", deviceID), zap.String("incoming event", incomingEvent.TransactionUUID), zap.String("comparison event", eventErr.Event().TransactionUUID))
			return
		}
	}

	t.logger.Error(msg, zap.Error(err), zap.String("deviceID", deviceID), zap.String("incoming event", incomingEvent.TransactionUUID))
}
