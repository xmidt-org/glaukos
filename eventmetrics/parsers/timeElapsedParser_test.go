package parsers

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/interpreter/validation"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
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
				assert.Equal(tc.config.Name, parser.Name())
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

func TestFixConfig(t *testing.T) {
	defaultTimeDuration := -2 * time.Hour
	tests := []struct {
		description    string
		config         TimeElapsedConfig
		expectedConfig TimeElapsedConfig
	}{
		{
			description: "no changes",
			config: TimeElapsedConfig{
				Name:            "test",
				SearchedSession: "previous",
				IncomingEvent: EventConfig{
					Regex:          ".*/online/",
					CalculateUsing: "birthdate",
					ValidFrom:      -30 * time.Minute,
				},
				SearchedEvent: EventConfig{
					Regex:          ".*/offline/",
					CalculateUsing: "boot-time",
					ValidFrom:      -2 * time.Minute,
				},
			},
			expectedConfig: TimeElapsedConfig{
				Name:            "TEP_test",
				SearchedSession: "previous",
				IncomingEvent: EventConfig{
					Regex:          ".*/online/",
					CalculateUsing: "birthdate",
					ValidFrom:      -30 * time.Minute,
				},
				SearchedEvent: EventConfig{
					Regex:          ".*/offline/",
					CalculateUsing: "boot-time",
					ValidFrom:      -2 * time.Minute,
				},
			},
		},
		{
			description: "spaces",
			config: TimeElapsedConfig{
				Name:            "test parser spaces",
				SearchedSession: "previous",
				IncomingEvent: EventConfig{
					Regex:          ".*/online/",
					CalculateUsing: "birthdate",
					ValidFrom:      -30 * time.Minute,
				},
				SearchedEvent: EventConfig{
					Regex:          ".*/offline/",
					CalculateUsing: "boot-time",
					ValidFrom:      -2 * time.Minute,
				},
			},
			expectedConfig: TimeElapsedConfig{
				Name:            "TEP_test_parser_spaces",
				SearchedSession: "previous",
				IncomingEvent: EventConfig{
					Regex:          ".*/online/",
					CalculateUsing: "birthdate",
					ValidFrom:      -30 * time.Minute,
				},
				SearchedEvent: EventConfig{
					Regex:          ".*/offline/",
					CalculateUsing: "boot-time",
					ValidFrom:      -2 * time.Minute,
				},
			},
		},
		{
			description: "missing valid from",
			config: TimeElapsedConfig{
				Name:            "test",
				SearchedSession: "previous",
				IncomingEvent: EventConfig{
					Regex:          ".*/online/",
					CalculateUsing: "birthdate",
				},
				SearchedEvent: EventConfig{
					Regex:          ".*/offline/",
					CalculateUsing: "boot-time",
				},
			},
			expectedConfig: TimeElapsedConfig{
				Name:            "TEP_test",
				SearchedSession: "previous",
				IncomingEvent: EventConfig{
					Regex:          ".*/online/",
					CalculateUsing: "birthdate",
					ValidFrom:      defaultTimeDuration,
				},
				SearchedEvent: EventConfig{
					Regex:          ".*/offline/",
					CalculateUsing: "boot-time",
					ValidFrom:      defaultTimeDuration,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			config := fixConfig(tc.config, defaultTimeDuration)
			assert.Equal(tc.expectedConfig, config)
		})
	}
}

func TestCalculateTimeElapsed(t *testing.T) {
	t.Run("invalid incoming event", testInvalidIncomingEvent)
	t.Run("device id parse err", testDeviceIDErr)
	t.Run("finder err", testFinderErr)
	t.Run("test calculations", testCalculations)
}
func TestTimeElapsedParseErr(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	logger := log.NewNopLogger()
	mockVal := new(mockValidator)
	mockClient := new(mockEventClient)
	mockFinder := new(mockFinder)
	mockClient.On("GetEvents", mock.Anything).Return([]interpreter.Event{})

	tests := []struct {
		description          string
		incomingEvent        interpreter.Event
		oldEvent             interpreter.Event
		incomingCalculateBy  TimeLocation
		oldCalculateBy       TimeLocation
		expectedUnparsable   float64
		expectedAddToMetrics bool
	}{
		{
			description: "invalid calculated time",
			incomingEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/some-event/1614265173",
				Birthdate:   now.UnixNano(),
			},
			oldEvent: interpreter.Event{
				Birthdate: now.Add(time.Hour).UnixNano(),
			},
			oldCalculateBy:      Birthdate,
			incomingCalculateBy: Birthdate,
		},
		{
			description: "no hardware key",
			incomingEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/some-event/1614265173",
				Metadata:    map[string]string{hardwareMetadataKey: "hardware"},
				Birthdate:   now.UnixNano(),
			},
			oldEvent: interpreter.Event{
				Birthdate: now.Add(-1 * time.Hour).UnixNano(),
			},
			oldCalculateBy:      Birthdate,
			incomingCalculateBy: Birthdate,
			expectedUnparsable:  1.0,
		},
		{
			description: "no firmware key",
			incomingEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/some-event/1614265173",
				Metadata:    map[string]string{firmwareMetadataKey: "firmware"},
				Birthdate:   now.UnixNano(),
			},
			oldEvent: interpreter.Event{
				Birthdate: now.Add(-1 * time.Hour).UnixNano(),
			},
			oldCalculateBy:      Birthdate,
			incomingCalculateBy: Birthdate,
			expectedUnparsable:  1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			p := xmetricstest.NewProvider(&xmetrics.Options{})
			m := Measures{
				UnparsableEventsCount: p.NewCounter("unparsable_events"),
				TimeElapsedHistograms: make(map[string]metrics.Histogram),
			}
			mockVal.On("Valid", tc.incomingEvent).Return(true, nil)
			mockFinder.On("Find", mock.Anything, tc.incomingEvent).Return(tc.oldEvent, nil).Once()
			parser := TimeElapsedParser{
				searchedEvent: EventInfo{CalculateUsing: tc.oldCalculateBy, Validator: mockVal},
				incomingEvent: EventInfo{CalculateUsing: tc.incomingCalculateBy, Validator: mockVal},
				logger:        logger,
				client:        mockClient,
				finder:        mockFinder,
				measures:      m,
				name:          "TEP_test",
			}

			parser.Parse(tc.incomingEvent)
			p.Assert(t, "unparsable_events", parserLabel, parser.name, reasonLabel, errNoFwHwLabel)(xmetricstest.Value(tc.expectedUnparsable))
		})
	}
}

func TestLogErrWithEventDetails(t *testing.T) {
	tests := []struct {
		description     string
		event           interpreter.Event
		err             error
		expectedJSONLog []string
	}{
		{
			description: "non error with event",
			err:         errors.New("test error"),
			event: interpreter.Event{
				Destination:     "event:device-status/mac:112233445566/some-event/1614265173",
				TransactionUUID: "incomingEventUUID",
			},
			expectedJSONLog: []string{"mac:112233445566", "test error", "incomingEventUUID"},
		},
		{
			description: "non error with event",
			err: testErrorWithEvent{
				err:   errors.New("test error"),
				event: interpreter.Event{TransactionUUID: "oldEventUUID"},
			},
			event: interpreter.Event{
				Destination:     "event:device-status/mac:112233445566/some-event/1614265173",
				TransactionUUID: "incomingEventUUID",
			},
			expectedJSONLog: []string{"mac:112233445566", "test error", "incomingEventUUID", "oldEventUUID"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			buf := &bytes.Buffer{}
			logger := log.NewJSONLogger(buf)
			parser := TimeElapsedParser{
				logger: logger,
			}
			parser.logErrWithEventDetails(tc.err, tc.event)
			logStr := buf.String()
			for _, logVal := range tc.expectedJSONLog {
				assert.Contains(logStr, logVal)
			}
		})
	}
}

func testInvalidIncomingEvent(t *testing.T) {
	assert := assert.New(t)
	mockVal := new(mockValidator)
	logger := log.NewNopLogger()
	invalidEventErr := errors.New("invalid event")
	incomingEvent := interpreter.Event{
		Destination: "some-destination",
	}
	parser := TimeElapsedParser{
		incomingEvent: EventInfo{Validator: mockVal},
		logger:        logger,
	}
	mockVal.On("Valid", incomingEvent).Return(false, validation.ErrInvalidEventType).Once()
	timeElapsed, err := parser.calculateTimeElapsed(incomingEvent)
	assert.Equal(-1.0, timeElapsed)
	assert.Equal(validation.ErrInvalidEventType, err)

	mockVal.On("Valid", incomingEvent).Return(false, invalidEventErr).Once()
	timeElapsed, err = parser.calculateTimeElapsed(incomingEvent)
	assert.Equal(-1.0, timeElapsed)
	assert.Equal(invalidEventErr, err)
}

func testDeviceIDErr(t *testing.T) {
	assert := assert.New(t)
	mockVal := new(mockValidator)
	logger := log.NewNopLogger()
	incomingEvent := interpreter.Event{
		Destination: "some-destination",
	}
	parser := TimeElapsedParser{
		incomingEvent: EventInfo{Validator: mockVal},
		logger:        logger,
	}
	mockVal.On("Valid", incomingEvent).Return(true, nil).Once()
	timeElapsed, err := parser.calculateTimeElapsed(incomingEvent)
	assert.Equal(-1.0, timeElapsed)
	assert.Equal(interpreter.ErrParseDeviceID, err)
}

func testFinderErr(t *testing.T) {
	assert := assert.New(t)
	logger := log.NewNopLogger()
	testErr := errors.New("test error")
	incomingEvent := interpreter.Event{
		Destination: "event:device-status/mac:112233445566/fully-manageable",
	}

	mockVal := new(mockValidator)
	mockClient := new(mockEventClient)
	mockFinder := new(mockFinder)
	mockVal.On("Valid", incomingEvent).Return(true, nil)
	mockClient.On("GetEvents", mock.Anything).Return([]interpreter.Event{})
	mockFinder.On("Find", mock.Anything, mock.Anything).Return(interpreter.Event{}, testErr)

	parser := TimeElapsedParser{
		incomingEvent: EventInfo{Validator: mockVal},
		logger:        logger,
		client:        mockClient,
		finder:        mockFinder,
	}

	timeElapsed, err := parser.calculateTimeElapsed(incomingEvent)
	assert.Equal(-1.0, timeElapsed)
	assert.Equal(testErr, err)
}

func testCalculations(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	logger := log.NewNopLogger()
	mockVal := new(mockValidator)
	mockClient := new(mockEventClient)
	mockFinder := new(mockFinder)
	mockClient.On("GetEvents", mock.Anything).Return([]interpreter.Event{})

	tests := []struct {
		description         string
		oldEvent            interpreter.Event
		newEvent            interpreter.Event
		oldEventCalculate   TimeLocation
		newEventCalculate   TimeLocation
		expectedTimeElapsed float64
		expectedErr         error
	}{
		{
			description: "valid time elapsed: boot-time - boot-time",
			oldEvent: interpreter.Event{
				Metadata:  map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Add(-30 * time.Second).Unix())},
				Birthdate: now.Add(-10 * time.Second).UnixNano(),
			},
			newEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata:    map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Unix())},
				Birthdate:   now.Add(-5 * time.Second).UnixNano(),
			},
			oldEventCalculate:   Boottime,
			newEventCalculate:   Boottime,
			expectedTimeElapsed: 30.0,
		},
		{
			description: "valid time elapsed: birthdate - birthdate",
			oldEvent: interpreter.Event{
				Metadata:  map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Add(-30 * time.Second).Unix())},
				Birthdate: now.Add(-10 * time.Second).UnixNano(),
			},
			newEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata:    map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Unix())},
				Birthdate:   now.Add(-5 * time.Second).UnixNano(),
			},
			oldEventCalculate:   Birthdate,
			newEventCalculate:   Birthdate,
			expectedTimeElapsed: 5.0,
		},
		{
			description: "valid time elapsed: birthdate - boot-time",
			oldEvent: interpreter.Event{
				Metadata:  map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Add(-30 * time.Second).Unix())},
				Birthdate: now.Add(-10 * time.Second).UnixNano(),
			},
			newEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata:    map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Unix())},
				Birthdate:   now.Add(-5 * time.Second).UnixNano(),
			},
			oldEventCalculate:   Boottime,
			newEventCalculate:   Birthdate,
			expectedTimeElapsed: 25.0,
		},
		{
			description: "valid time elapsed: boot-time - birthdate",
			oldEvent: interpreter.Event{
				Metadata:  map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Add(-30 * time.Second).Unix())},
				Birthdate: now.Add(-10 * time.Second).UnixNano(),
			},
			newEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata:    map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Unix())},
				Birthdate:   now.Add(-5 * time.Second).UnixNano(),
			},
			oldEventCalculate:   Birthdate,
			newEventCalculate:   Boottime,
			expectedTimeElapsed: 10.0,
		},
		{
			description: "invalid time elapsed: negative time",
			oldEvent: interpreter.Event{
				Metadata:  map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Add(-30 * time.Second).Unix())},
				Birthdate: now.Add(-10 * time.Second).UnixNano(),
			},
			newEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata:    map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Unix())},
				Birthdate:   now.Add(-1 * time.Hour).UnixNano(),
			},
			oldEventCalculate:   Birthdate,
			newEventCalculate:   Birthdate,
			expectedTimeElapsed: -1.0,
			expectedErr:         TimeElapsedCalculationErr{timeElapsed: now.Add(-1 * time.Hour).Sub(now.Add(-10 * time.Second)).Seconds()},
		},
		{
			description: "invalid time elapsed: old event missing timestamp",
			oldEvent: interpreter.Event{
				Metadata:  map[string]string{},
				Birthdate: now.Add(-10 * time.Second).UnixNano(),
			},
			newEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata:    map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Unix())},
				Birthdate:   now.Add(-1 * time.Hour).UnixNano(),
			},
			oldEventCalculate:   Boottime,
			newEventCalculate:   Birthdate,
			expectedTimeElapsed: -1.0,
			expectedErr:         TimeElapsedCalculationErr{timeElapsed: 0},
		},
		{
			description: "invalid time elapsed: new event missing timestamp",
			oldEvent: interpreter.Event{
				Metadata:  map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Add(-30 * time.Second).Unix())},
				Birthdate: now.Add(-10 * time.Second).UnixNano(),
			},
			newEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata:    map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Unix())},
			},
			oldEventCalculate:   Boottime,
			newEventCalculate:   Birthdate,
			expectedTimeElapsed: -1.0,
			expectedErr:         TimeElapsedCalculationErr{timeElapsed: 0},
		},
		{
			description: "empty old event",
			oldEvent:    interpreter.Event{},
			newEvent: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata:    map[string]string{interpreter.BootTimeKey: fmt.Sprint(now.Unix())},
			},
			oldEventCalculate:   Boottime,
			newEventCalculate:   Birthdate,
			expectedTimeElapsed: -1.0,
			expectedErr:         TimeElapsedCalculationErr{timeElapsed: 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			mockVal.On("Valid", tc.newEvent).Return(true, nil)
			mockFinder.On("Find", mock.Anything, tc.newEvent).Return(tc.oldEvent, nil).Once()
			parser := TimeElapsedParser{
				searchedEvent: EventInfo{CalculateUsing: tc.oldEventCalculate, Validator: mockVal},
				incomingEvent: EventInfo{CalculateUsing: tc.newEventCalculate, Validator: mockVal},
				logger:        logger,
				client:        mockClient,
				finder:        mockFinder,
			}

			timeElapsed, err := parser.calculateTimeElapsed(tc.newEvent)
			assert.Equal(tc.expectedTimeElapsed, timeElapsed)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.Contains(err.Error(), tc.expectedErr.Error())
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
