package parsers

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
)

type testFinder struct {
	events        []interpreter.Event
	incomingEvent interpreter.Event
	expectedEvent interpreter.Event
	err           error
}

func TestNewTimeElapsedParser(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTime := func() time.Time { return now }
	logger := log.NewNopLogger()
	mockClient := new(mockEventClient)

	tests := []struct {
		description      string
		config           TimeElapsedConfig
		expectedIncoming EventInfo
		expectedSearched EventInfo
		testFinders      []testFinder
		expectedErr      error
	}{
		{
			description: "incoming event invalid regex",
			config: TimeElapsedConfig{
				IncomingEvent: EventConfig{
					Regex: "[",
				},
			},
			expectedErr: errInvalidRegex,
		},
		{
			description: "no searched event in config",
			config: TimeElapsedConfig{
				IncomingEvent: EventConfig{
					Regex:          ".*/some-event/",
					CalculateUsing: "boot-time",
					ValidFrom:      -2 * time.Hour,
				},
			},
			expectedIncoming: EventInfo{
				Regex:          regexp.MustCompile(".*/some-event/"),
				CalculateUsing: Birthdate,
			},
			expectedSearched: EventInfo{
				Regex:          regexp.MustCompile(".*/some-event/"),
				CalculateUsing: Boottime,
			},
			testFinders: []testFinder{
				newerBootTimeTestFinder("event:device-status/mac:112233445566/some-event/1614265173"),
				duplicateEventTestFinder("event:device-status/mac:112233445566/some-event/1614265173"),
				historyInteratorTestFinder("event:device-status/mac:112233445566/some-event/1614265173", "event:device-status/mac:112233445566/some-event/1614265173"),
			},
		},
		{
			description: "past session searched event",
			config: TimeElapsedConfig{
				IncomingEvent: EventConfig{
					Regex:          ".*/some-event/",
					CalculateUsing: "boot-time",
					ValidFrom:      -2 * time.Hour,
				},
				SearchedEvent: EventConfig{
					Regex:          ".*/old-event/",
					CalculateUsing: "boot-time",
					ValidFrom:      -2 * time.Hour,
				},
				SearchedSession: "previous",
			},
			expectedIncoming: EventInfo{
				Regex:          regexp.MustCompile(".*/some-event/"),
				CalculateUsing: Boottime,
			},
			expectedSearched: EventInfo{
				Regex:          regexp.MustCompile(".*/old-event/"),
				CalculateUsing: Boottime,
			},
			testFinders: []testFinder{
				newerBootTimeTestFinder("event:device-status/mac:112233445566/some-event/1614265173"),
				duplicateEventTestFinder("event:device-status/mac:112233445566/some-event/1614265173"),
				pastSessionTestFinder("event:device-status/mac:112233445566/some-event/1614265173", now, "event:device-status/mac:112233445566/old-event/1614265173"),
			},
		},
		{
			description: "past session searched event",
			config: TimeElapsedConfig{
				IncomingEvent: EventConfig{
					Regex:          ".*/some-event/",
					CalculateUsing: "boot-time",
					ValidFrom:      -2 * time.Hour,
				},
				SearchedEvent: EventConfig{
					Regex:          ".*/old-event/",
					CalculateUsing: "boot-time",
					ValidFrom:      -2 * time.Hour,
				},
				SearchedSession: "current",
			},
			expectedIncoming: EventInfo{
				Regex:          regexp.MustCompile(".*/some-event/"),
				CalculateUsing: Boottime,
			},
			expectedSearched: EventInfo{
				Regex:          regexp.MustCompile(".*/old-event/"),
				CalculateUsing: Boottime,
			},
			testFinders: []testFinder{
				newerBootTimeTestFinder("event:device-status/mac:112233445566/some-event/1614265173"),
				duplicateEventTestFinder("event:device-status/mac:112233445566/some-event/1614265173"),
				currentSessionTestFinder("event:device-status/mac:112233445566/some-event/1614265173", now, "event:device-status/mac:112233445566/old-event/1614265173"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			parser, err := NewTimeElapsedParser(tc.config, mockClient, logger, Measures{}, currTime)
			if tc.expectedErr != nil {
				assert.Nil(parser)
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			} else {
				assert.NotNil(parser)
				assert.Nil(err)
				assert.Equal(tc.config.Name, parser.name)
				assert.Equal(mockClient, parser.client)
				assert.Equal(logger, parser.logger)
				assert.True(testEventInfoEqual(parser.incomingEvent, tc.expectedIncoming))
				assert.True(testEventInfoEqual(parser.searchedEvent, tc.expectedSearched))
				assert.NotNil(parser.finder)
				for _, finder := range tc.testFinders {
					eventFound, foundErr := parser.finder.Find(finder.events, finder.incomingEvent)
					assert.Equal(finder.expectedEvent, eventFound)
					if finder.err == nil || foundErr == nil {
						assert.Equal(finder.err, foundErr)
					} else {
						assert.Contains(foundErr.Error(), finder.err.Error())
					}

				}
			}
		})
	}
}

func historyInteratorTestFinder(incomingDest string, oldEventDest string) testFinder {
	incomingEvent := interpreter.Event{
		Destination: incomingDest,
		Metadata: map[string]string{
			interpreter.BootTimeKey: "60",
		},
		TransactionUUID: "newEvent",
		Birthdate:       70,
	}

	events := []interpreter.Event{
		interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: "50",
			},
			TransactionUUID: "oldEvent",
			Birthdate:       50,
		},
		interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: "50",
			},
			TransactionUUID: "oldEvent",
			Birthdate:       50,
		},
		interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: "40",
			},
			TransactionUUID: "oldEvent",
			Birthdate:       50,
		},
	}

	return testFinder{
		incomingEvent: incomingEvent,
		events:        events,
		expectedEvent: incomingEvent,
	}
}

func currentSessionTestFinder(incomingDest string, incomingEventTime time.Time, oldEventDest string) testFinder {
	incomingEvent := interpreter.Event{
		Destination: incomingDest,
		Metadata: map[string]string{
			interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Unix()),
		},
		TransactionUUID: "newEvent",
		Birthdate:       incomingEventTime.UnixNano(),
	}

	events := []interpreter.Event{
		interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Add(-2 * time.Minute).Unix()),
			},
			TransactionUUID: "oldEvent",
			Birthdate:       incomingEventTime.Add(-2 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Unix()),
			},
			TransactionUUID: "oldEvent",
			Birthdate:       incomingEventTime.UnixNano(),
		},
		interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Add(-3 * time.Minute).Unix()),
			},
			TransactionUUID: "oldEvent",
			Birthdate:       incomingEventTime.Add(-3 * time.Minute).UnixNano(),
		},
	}

	return testFinder{
		incomingEvent: incomingEvent,
		events:        events,
		expectedEvent: interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Unix()),
			},
			TransactionUUID: "oldEvent",
			Birthdate:       incomingEventTime.UnixNano(),
		},
	}
}

func pastSessionTestFinder(incomingDest string, incomingEventTime time.Time, oldEventDest string) testFinder {
	incomingEvent := interpreter.Event{
		Destination: incomingDest,
		Metadata: map[string]string{
			interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Unix()),
		},
		TransactionUUID: "newEvent",
		Birthdate:       incomingEventTime.UnixNano(),
	}

	events := []interpreter.Event{
		interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Add(-2 * time.Minute).Unix()),
			},
			TransactionUUID: "oldEvent",
			Birthdate:       incomingEventTime.Add(-2 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Add(-1 * time.Minute).Unix()),
			},
			TransactionUUID: "oldEvent",
			Birthdate:       incomingEventTime.Add(-1 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Add(-3 * time.Minute).Unix()),
			},
			TransactionUUID: "oldEvent",
			Birthdate:       incomingEventTime.Add(-3 * time.Minute).UnixNano(),
		},
	}

	return testFinder{
		incomingEvent: incomingEvent,
		events:        events,
		expectedEvent: interpreter.Event{
			Destination: oldEventDest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(incomingEventTime.Add(-1 * time.Minute).Unix()),
			},
			TransactionUUID: "oldEvent",
			Birthdate:       incomingEventTime.Add(-1 * time.Minute).UnixNano(),
		},
	}
}

func duplicateEventTestFinder(dest string) testFinder {
	incomingEvent := interpreter.Event{
		Destination: dest,
		Metadata: map[string]string{
			interpreter.BootTimeKey: "60",
		},
		TransactionUUID: "newEvent",
		Birthdate:       70,
	}

	events := []interpreter.Event{
		interpreter.Event{
			Destination: dest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: "50",
			},
			TransactionUUID: "oldEvent",
			Birthdate:       50,
		},
		interpreter.Event{
			Destination: dest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: "60",
			},
			TransactionUUID: "oldEvent",
			Birthdate:       50,
		},
	}

	return testFinder{
		incomingEvent: incomingEvent,
		events:        events,
		expectedEvent: interpreter.Event{},
		err:           history.ComparatorErr{},
	}
}

func newerBootTimeTestFinder(dest string) testFinder {
	incomingEvent := interpreter.Event{
		Destination: dest,
		Metadata: map[string]string{
			interpreter.BootTimeKey: "60",
		},
		TransactionUUID: "newEvent",
	}

	events := []interpreter.Event{
		interpreter.Event{
			Destination: dest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: "50",
			},
			TransactionUUID: "oldEvent",
		},
		interpreter.Event{
			Destination: dest,
			Metadata: map[string]string{
				interpreter.BootTimeKey: "70",
			},
			TransactionUUID: "oldEvent",
		},
	}

	return testFinder{
		incomingEvent: incomingEvent,
		events:        events,
		expectedEvent: interpreter.Event{},
		err:           history.ComparatorErr{},
	}
}

func testEventInfoEqual(eventInfoOne EventInfo, eventInfoTwo EventInfo) bool {
	return eventInfoOne.CalculateUsing == eventInfoTwo.CalculateUsing && eventInfoOne.Regex.String() == eventInfoTwo.Regex.String()
}
