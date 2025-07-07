// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/touchstone/touchtest"
	"go.uber.org/zap"
)

func TestParseCalculationErr(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	const (
		hwVal = "hw"
		fwVal = "fw"
	)

	var (
		event = interpreter.Event{
			Destination: "event:device-status/mac:112233445566/fully-manageable",
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				hardwareMetadataKey:     hwVal,
				firmwareMetadataKey:     fwVal,
			},
			Birthdate: now.Add(-2 * time.Minute).UnixNano(),
		}

		client                  = new(mockEventClient)
		eventsParser            = new(mockEventsParser)
		validParserValidator    = new(mockParserValidator)
		validDurationCalculator = new(mockDurationCalculator)
	)

	client.On("GetEvents", mock.Anything).Return([]interpreter.Event{})
	eventsParser.On("Parse", mock.Anything, mock.Anything).Return([]interpreter.Event{interpreter.Event{}}, nil)
	validParserValidator.On("Validate", mock.Anything, mock.Anything).Return(true, nil)
	validDurationCalculator.On("Calculate", mock.Anything, mock.Anything).Return(nil)

	tests := []struct {
		description string
		err         error
		expectedInc bool
	}{
		{
			description: "random error",
			err:         errors.New("test error"),
			expectedInc: true,
		},
		{
			description: "event not found",
			err:         errEventNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				m = Measures{
					RebootUnparsableCount: prometheus.NewCounterVec(
						prometheus.CounterOpts{
							Name: "rebootUnparsableEvents",
							Help: "rebootUnparsableEvents",
						},
						[]string{firmwareLabel, hardwareLabel, partnerIDLabel, reasonLabel},
					),
					TotalUnparsableCount: prometheus.NewCounterVec(
						prometheus.CounterOpts{
							Name: "totalUnparsableEvents",
							Help: "totalUnparsableEvents",
						},
						[]string{parserLabel},
					),
				}
			)

			invalidDurationCalculator := new(mockDurationCalculator)
			invalidDurationCalculator.On("Calculate", mock.Anything, mock.Anything).Return(tc.err)

			parser := RebootDurationParser{
				name:                 "test_reboot_parser",
				logger:               zap.NewNop(),
				measures:             m,
				relevantEventsParser: eventsParser,
				client:               client,
				parserValidators:     []ParserValidator{validParserValidator},
				calculators:          []DurationCalculator{validDurationCalculator, invalidDurationCalculator},
			}

			parser.Parse(event)
		})
	}

}

func TestParseValidationErr(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	var (
		hwVal = "hw"
		fwVal = "fw"
		event = interpreter.Event{
			Destination: "event:device-status/mac:112233445566/fully-manageable",
			Metadata: map[string]string{
				hardwareMetadataKey:     hwVal,
				firmwareMetadataKey:     fwVal,
				interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
			},
		}
		m = Measures{
			RebootUnparsableCount: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "rebootUnparsableEvents",
					Help: "rebootUnparsableEvents",
				},
				[]string{firmwareLabel, hardwareLabel, partnerIDLabel, reasonLabel},
			),
			TotalUnparsableCount: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "totalUnparsableEvents",
					Help: "totalUnparsableEvents",
				},
				[]string{parserLabel},
			),
		}

		client                 = new(mockEventClient)
		eventsParser           = new(mockEventsParser)
		validParserValidator   = new(mockParserValidator)
		invalidParserValidator = new(mockParserValidator)
	)

	client.On("GetEvents", mock.Anything).Return([]interpreter.Event{})
	eventsParser.On("Parse", mock.Anything, mock.Anything).Return([]interpreter.Event{interpreter.Event{}}, nil)
	validParserValidator.On("Validate", mock.Anything, mock.Anything).Return(true, nil)
	invalidParserValidator.On("Validate", mock.Anything, mock.Anything).Return(false, errors.New("validation err"))

	rebootParser := RebootDurationParser{
		name:                 "test_reboot_parser",
		client:               client,
		relevantEventsParser: eventsParser,
		parserValidators:     []ParserValidator{validParserValidator, invalidParserValidator},
		measures:             m,
		logger:               zap.NewNop(),
	}

	rebootParser.Parse(event)
}

func TestParseNotFullyManageable(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	event := interpreter.Event{
		Destination: "event:device-status/mac:112233445566/online",
		Metadata: map[string]string{
			hardwareMetadataKey:     "hw",
			firmwareMetadataKey:     "fw",
			interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
		},
	}

	expectedTotalUnparsableCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "totalUnparsableEvents",
			Help: "totalUnparsableEvents",
		},
		[]string{parserLabel},
	)
	expectedRebootUnparsableCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rebootUnparsableEvents",
			Help: "rebootUnparsableEvents",
		},
		[]string{firmwareLabel, hardwareLabel, reasonLabel},
	)

	m := Measures{
		TotalUnparsableCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "totalUnparsableEvents",
				Help: "totalUnparsableEvents",
			},
			[]string{parserLabel},
		),
		RebootUnparsableCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rebootUnparsableEvents",
				Help: "rebootUnparsableEvents",
			},
			[]string{firmwareLabel, hardwareLabel, reasonLabel},
		),
	}

	parser := RebootDurationParser{
		measures: m,
		name:     "test_reboot_parser",
		logger:   zap.NewNop(),
	}

	assert := assert.New(t)

	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedTotalUnparsableCounter)
	expectedRegistry.Register(expectedRebootUnparsableCounter)
	actualRegistry.Register(m.TotalUnparsableCount)
	actualRegistry.Register(m.RebootUnparsableCount)

	parser.Parse(event)
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(testAssert.GatherAndCompare(actualRegistry))
}

func TestParseFatalErr(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	hwVal := "hw"
	fwVal := "fw"
	invalidParser := new(mockEventsParser)
	validParser := new(mockEventsParser)
	client := new(mockEventClient)
	invalidParser.On("Parse", mock.Anything, mock.Anything).Return([]interpreter.Event{}, errors.New("test"))
	validParser.On("Parse", mock.Anything, mock.Anything).Return([]interpreter.Event{interpreter.Event{}}, nil)
	client.On("GetEvents", mock.Anything).Return([]interpreter.Event{})

	tests := []struct {
		description      string
		event            interpreter.Event
		cycleParser      EventsParser
		validationParser EventsParser
	}{
		{
			description: "not an event",
			event: interpreter.Event{
				Destination: "some-destination",
				Metadata: map[string]string{
					hardwareMetadataKey:     hwVal,
					firmwareMetadataKey:     fwVal,
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
		},
		{
			description: "no device id",
			event: interpreter.Event{
				Destination: "event:device-status/some-id/online",
				Metadata: map[string]string{
					hardwareMetadataKey:     hwVal,
					firmwareMetadataKey:     fwVal,
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
		},
		{
			description: "no boot-time",
			event: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata: map[string]string{
					hardwareMetadataKey: hwVal,
					firmwareMetadataKey: fwVal,
				},
			},
		},
		{
			description: "err getting cycle",
			event: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata: map[string]string{
					hardwareMetadataKey:     hwVal,
					firmwareMetadataKey:     fwVal,
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			cycleParser: invalidParser,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				totalUnparsableCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "totalUnparsableEvents",
						Help: "totalUnparsableEvents",
					},
					[]string{parserLabel},
				)
				rebootUnparsableCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "rebootUnparsableEvents",
						Help: "rebootUnparsableEvents",
					},
					[]string{firmwareLabel, hardwareLabel, partnerIDLabel, reasonLabel},
				)
			)

			m := Measures{
				RebootUnparsableCount: rebootUnparsableCounter,
				TotalUnparsableCount:  totalUnparsableCounter,
			}

			parser := RebootDurationParser{
				measures:             m,
				name:                 "test_reboot_parser",
				logger:               zap.NewNop(),
				relevantEventsParser: tc.cycleParser,
				client:               client,
			}

			parser.Parse(tc.event)
		})
	}
}

func TestParseNoFWHWErr(t *testing.T) {
	hwVal := "hw"
	fwVal := "fw"
	destination := "event:device-status/mac:112233445566/online"
	tests := []struct {
		description string
		event       interpreter.Event
		expectErr   bool
	}{
		{
			description: "no fw",
			event: interpreter.Event{
				Destination: destination,
				Metadata: map[string]string{
					hardwareMetadataKey: hwVal,
				},
			},
			expectErr: true,
		},
		{
			description: "no hw",
			event: interpreter.Event{
				Destination: destination,
				Metadata: map[string]string{
					firmwareMetadataKey: fwVal,
				},
			},
			expectErr: true,
		},
		{
			description: "no hw/fw",
			event: interpreter.Event{
				Destination: destination,
				Metadata:    map[string]string{},
			},
			expectErr: true,
		},
		{
			description: "valid",
			event: interpreter.Event{
				Destination: destination,
				Metadata: map[string]string{
					firmwareMetadataKey: fwVal,
					hardwareMetadataKey: hwVal,
				},
			},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			m := Measures{
				RebootUnparsableCount: prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "unparsable",
						Help: "unparsable",
					},
					[]string{firmwareLabel, hardwareLabel, reasonLabel},
				),
			}

			parser := RebootDurationParser{
				measures: m,
				logger:   zap.NewNop(),
			}

			parser.Parse(tc.event)
			if tc.expectErr {
				assert.Equal(1.0, testutil.ToFloat64(m.RebootUnparsableCount))
			}
		})
	}
}

func TestRebootDurationParserName(t *testing.T) {
	name := "testRebootParser"
	parser := RebootDurationParser{
		name: name,
	}
	assert.Equal(t, name, parser.Name())
}

func TestBasicChecks(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	tests := []struct {
		description   string
		event         interpreter.Event
		expectedValid bool
	}{
		{
			description: "invalid boot-time",
			event: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{
					interpreter.BootTimeKey: "-1",
				},
			},
			expectedValid: false,
		},
		{
			description: "no boot-time",
			event: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/online",
				Metadata:    map[string]string{},
			},
			expectedValid: false,
		},
		{
			description: "no device id",
			event: interpreter.Event{
				Destination: "some destination",
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			expectedValid: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			parser := RebootDurationParser{
				logger: zap.NewNop(),
			}
			valid := parser.basicChecks(tc.event)
			assert.Equal(t, tc.expectedValid, valid)
		})
	}
}

func TestGetEvents(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	logger := zap.NewNop()
	testErr := errors.New("test")
	client := new(mockEventClient)
	client.On("GetEvents", mock.Anything).Return([]interpreter.Event{})

	events := []interpreter.Event{
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-60 * time.Minute).Unix()),
			},
			Birthdate: now.Add(-50 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-60 * time.Minute).Unix()),
			},
			Birthdate: now.Add(-40 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-70 * time.Minute).Unix()),
			},
			Birthdate: now.Add(-63 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-60 * time.Minute).Unix()),
			},
			Birthdate: now.Add(-70 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Birthdate: now.Add(-40 * time.Minute).UnixNano(),
		},
	}

	sortedEvents := make([]interpreter.Event, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(a, b int) bool {
		boottimeA, _ := sortedEvents[a].BootTime()
		boottimeB, _ := sortedEvents[b].BootTime()
		if boottimeA != boottimeB {
			return boottimeA > boottimeB
		}
		return sortedEvents[a].Birthdate > sortedEvents[b].Birthdate
	})

	tests := []struct {
		description    string
		currentEvent   interpreter.Event
		parsedEvents   []interpreter.Event
		parseErr       error
		expectedEvents []interpreter.Event
		expectedErr    bool
	}{
		{
			description: "valid",
			currentEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/someEvent",
			},
			parsedEvents:   events,
			expectedEvents: sortedEvents,
		},
		{
			description: "valid empty",
			currentEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/someEvent",
			},
			parsedEvents:   []interpreter.Event{},
			expectedEvents: []interpreter.Event{},
		},
		{
			description: "device id err",
			currentEvent: interpreter.Event{
				Destination: "event:device-status/someEvent",
			},
			expectedEvents: []interpreter.Event{},
			expectedErr:    true,
		},
		{
			description: "err parsing",
			currentEvent: interpreter.Event{
				Destination: "event:device-status/someEvent",
			},
			parsedEvents:   []interpreter.Event{},
			parseErr:       testErr,
			expectedEvents: []interpreter.Event{},
			expectedErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			eventsParser := new(mockEventsParser)
			eventsParser.On("Parse", mock.Anything, mock.Anything).Return(tc.parsedEvents, tc.parseErr)
			rebootParser := RebootDurationParser{
				client:               client,
				relevantEventsParser: eventsParser,
				logger:               logger,
			}

			returnedEvents, err := rebootParser.getEvents(tc.currentEvent)
			assert.Equal(tc.expectedEvents, returnedEvents)
			if tc.expectedErr {
				assert.NotNil(err)
			} else {
				assert.Nil(err)
			}

		})
	}
}
