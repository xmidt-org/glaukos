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
				expectedTotalUnparsableCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "totalUnparsableEvents",
						Help: "totalUnparsableEvents",
					},
					[]string{parserLabel},
				)
				expectedRebootUnparsableCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "rebootUnparsableEvents",
						Help: "rebootUnparsableEvents",
					},
					[]string{firmwareLabel, hardwareLabel, reasonLabel},
				)

				m = Measures{
					RebootUnparsableCount: prometheus.NewCounterVec(
						prometheus.CounterOpts{
							Name: "rebootUnparsableEvents",
							Help: "rebootUnparsableEvents",
						},
						[]string{firmwareLabel, hardwareLabel, reasonLabel},
					),
					TotalUnparsableEvents: prometheus.NewCounterVec(
						prometheus.CounterOpts{
							Name: "totalUnparsableEvents",
							Help: "totalUnparsableEvents",
						},
						[]string{parserLabel},
					),
				}
			)

			assert := assert.New(t)
			expectedRegistry := prometheus.NewPedanticRegistry()
			actualRegistry := prometheus.NewPedanticRegistry()
			expectedRegistry.Register(expectedTotalUnparsableCounter)
			expectedRegistry.Register(expectedRebootUnparsableCounter)
			actualRegistry.Register(m.TotalUnparsableEvents)
			actualRegistry.Register(m.RebootUnparsableCount)

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

			if tc.expectedInc {
				expectedTotalUnparsableCounter.WithLabelValues("test_reboot_parser").Inc()
				expectedRebootUnparsableCounter.WithLabelValues(fwVal, hwVal, calculationErrReason).Inc()
			}

			testAssert := touchtest.New(t)
			testAssert.Expect(expectedRegistry)
			assert.True(testAssert.GatherAndCompare(actualRegistry))

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
				[]string{firmwareLabel, hardwareLabel, reasonLabel},
			),
			TotalUnparsableEvents: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Name: "totalUnparsableEvents",
					Help: "totalUnparsableEvents",
				},
				[]string{parserLabel},
			),
		}

		expectedTotalUnparsableCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "totalUnparsableEvents",
				Help: "totalUnparsableEvents",
			},
			[]string{parserLabel},
		)
		expectedRebootUnparsableCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rebootUnparsableEvents",
				Help: "rebootUnparsableEvents",
			},
			[]string{firmwareLabel, hardwareLabel, reasonLabel},
		)

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

	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedTotalUnparsableCounter)
	expectedRegistry.Register(expectedRebootUnparsableCounter)
	actualRegistry.Register(m.TotalUnparsableEvents)
	actualRegistry.Register(m.RebootUnparsableCount)

	expectedTotalUnparsableCounter.WithLabelValues("test_reboot_parser").Inc()
	expectedRebootUnparsableCounter.WithLabelValues(fwVal, hwVal, validationErrReason).Inc()

	rebootParser.Parse(event)
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(t, testAssert.GatherAndCompare(actualRegistry))

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
		TotalUnparsableEvents: prometheus.NewCounterVec(
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
	actualRegistry.Register(m.TotalUnparsableEvents)
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
				assert                 = assert.New(t)
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
					[]string{firmwareLabel, hardwareLabel, reasonLabel},
				)
				expectedTotalUnparsableCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "totalUnparsableEvents",
						Help: "totalUnparsableEvents",
					},
					[]string{parserLabel},
				)
				expectedRebootUnparsableCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "rebootUnparsableEvents",
						Help: "rebootUnparsableEvents",
					},
					[]string{firmwareLabel, hardwareLabel, reasonLabel},
				)
			)

			m := Measures{
				RebootUnparsableCount: rebootUnparsableCounter,
				TotalUnparsableEvents: totalUnparsableCounter,
			}

			parser := RebootDurationParser{
				measures:             m,
				name:                 "test_reboot_parser",
				logger:               zap.NewNop(),
				relevantEventsParser: tc.cycleParser,
				client:               client,
			}

			expectedRegistry := prometheus.NewPedanticRegistry()
			actualRegistry := prometheus.NewPedanticRegistry()
			expectedRegistry.Register(expectedTotalUnparsableCounter)
			expectedRegistry.Register(expectedRebootUnparsableCounter)
			actualRegistry.Register(m.TotalUnparsableEvents)
			actualRegistry.Register(m.RebootUnparsableCount)

			expectedTotalUnparsableCounter.WithLabelValues("test_reboot_parser").Inc()
			expectedRebootUnparsableCounter.WithLabelValues(fwVal, hwVal, fatalErrReason).Inc()

			parser.Parse(tc.event)
			testAssert := touchtest.New(t)
			testAssert.Expect(expectedRegistry)
			assert.True(testAssert.GatherAndCompare(actualRegistry))
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

func TestAddToUnparsableCounters(t *testing.T) {
	firmwareVal := "firmware"
	hardwareVal := "hardware"
	reasonVal := "invalid"
	m := Measures{
		TotalUnparsableEvents: prometheus.NewCounterVec(
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
	}

	parser.addToUnparsableCounters(firmwareVal, hardwareVal, reasonVal)
	assert := assert.New(t)
	assert.Equal(1.0, testutil.ToFloat64(m.TotalUnparsableEvents))
	assert.Equal(1.0, testutil.ToFloat64(m.RebootUnparsableCount))
}

// func createCycleValidators(errs []error, numValid int) []history.CycleValidator {
// 	cycleValidators := make([]history.CycleValidator, len(errs)+numValid)
// 	for i := 0; i < len(errs)+numValid; i++ {
// 		var validator *mockCycleValidator
// 		if i < len(errs) {
// 			validator = new(mockCycleValidator)
// 			validator.On("Valid", mock.Anything).Return(false, errs[i])
// 		} else {
// 			validator = new(mockCycleValidator)
// 			validator.On("Valid", mock.Anything).Return(true, nil)
// 		}
// 		cycleValidators[i] = validator
// 	}

// 	return cycleValidators
// }

// func TestCalculateDurations(t *testing.T) {
// 	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
// 	assert.Nil(t, err)
// 	hwVal := "hw"
// 	fwVal := "fw"
// 	rebootVal := "reboot"
// 	tests := []struct {
// 		description               string
// 		event                     interpreter.Event
// 		finderErr                 error
// 		finderEvent               interpreter.Event
// 		expectedBootDurationErr   bool
// 		expectedRebootDurationErr bool
// 		expectedValid             bool
// 	}{
// 		{
// 			description: "boot calculation err",
// 			event: interpreter.Event{
// 				Metadata: map[string]string{
// 					interpreter.BootTimeKey: fmt.Sprint(now.Add(time.Minute).Unix()),
// 					hardwareMetadataKey:     hwVal,
// 					firmwareMetadataKey:     fwVal,
// 					rebootReasonMetadataKey: rebootVal,
// 				},
// 				Birthdate: now.UnixNano(),
// 			},
// 			finderEvent: interpreter.Event{
// 				Metadata: map[string]string{
// 					interpreter.BootTimeKey: fmt.Sprint(now.Add(time.Minute).Unix()),
// 				},
// 				Birthdate: now.Add(-2 * time.Minute).UnixNano(),
// 			},
// 			expectedBootDurationErr:   true,
// 			expectedRebootDurationErr: false,
// 			expectedValid:             false,
// 		},
// 		{
// 			description: "reboot calculation err",
// 			event: interpreter.Event{
// 				Metadata: map[string]string{
// 					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
// 					hardwareMetadataKey:     hwVal,
// 					firmwareMetadataKey:     fwVal,
// 					rebootReasonMetadataKey: rebootVal,
// 				},
// 				Birthdate: now.Add(time.Minute).UnixNano(),
// 			},
// 			finderErr: errors.New("cannot find event"),
// 			finderEvent: interpreter.Event{
// 				Metadata: map[string]string{
// 					interpreter.BootTimeKey: fmt.Sprint(now.Add(time.Minute).Unix()),
// 				},
// 				Birthdate: now.Add(-2 * time.Minute).UnixNano(),
// 			},
// 			expectedBootDurationErr:   false,
// 			expectedRebootDurationErr: true,
// 			expectedValid:             false,
// 		},
// 		{
// 			description: "all valid",
// 			event: interpreter.Event{
// 				Metadata: map[string]string{
// 					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
// 					hardwareMetadataKey:     hwVal,
// 					firmwareMetadataKey:     fwVal,
// 					rebootReasonMetadataKey: rebootVal,
// 				},
// 				Birthdate: now.Add(time.Minute).UnixNano(),
// 			},
// 			finderEvent: interpreter.Event{
// 				Metadata: map[string]string{
// 					interpreter.BootTimeKey: fmt.Sprint(now.Add(time.Minute).Unix()),
// 				},
// 				Birthdate: now.Add(-2 * time.Minute).UnixNano(),
// 			},
// 			expectedBootDurationErr:   false,
// 			expectedRebootDurationErr: false,
// 			expectedValid:             true,
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.description, func(t *testing.T) {
// 			var (
// 				assert                = assert.New(t)
// 				expectedRegistry      = prometheus.NewPedanticRegistry()
// 				actualRegistry        = prometheus.NewPedanticRegistry()
// 				expectedBootHistogram = prometheus.NewHistogramVec(
// 					prometheus.HistogramOpts{
// 						Name:    "boot_to_manageable",
// 						Help:    "boot_to_manageable",
// 						Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
// 					},
// 					[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
// 				)
// 				expectedRebootHistogram = prometheus.NewHistogramVec(
// 					prometheus.HistogramOpts{
// 						Name:    "reboot_to_manageable",
// 						Help:    "reboot_to_manageable",
// 						Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
// 					},
// 					[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
// 				)
// 				actualBootHistogram = prometheus.NewHistogramVec(
// 					prometheus.HistogramOpts{
// 						Name:    "boot_to_manageable",
// 						Help:    "boot_to_manageable",
// 						Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
// 					},
// 					[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
// 				)
// 				actualRebootHistogram = prometheus.NewHistogramVec(
// 					prometheus.HistogramOpts{
// 						Name:    "reboot_to_manageable",
// 						Help:    "reboot_to_manageable",
// 						Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
// 					},
// 					[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
// 				)
// 			)

// 			expectedRegistry.Register(expectedBootHistogram)
// 			expectedRegistry.Register(expectedRebootHistogram)
// 			actualRegistry.Register(actualBootHistogram)
// 			actualRegistry.Register(actualRebootHistogram)
// 			m := Measures{
// 				BootToManageableHistogram:   actualBootHistogram,
// 				RebootToManageableHistogram: actualRebootHistogram,
// 			}

// 			mockFinder := new(mockFinder)
// 			mockFinder.On("Find", mock.Anything, mock.Anything).Return(tc.finderEvent, tc.finderErr)
// 			parser := RebootDurationParser{
// 				logger:   zap.NewNop(),
// 				finder:   mockFinder,
// 				measures: m,
// 			}
// 			allValid := parser.calculateDurations([]interpreter.Event{}, tc.event)
// 			assert.Equal(tc.expectedValid, allValid)
// 			if !tc.expectedBootDurationErr {
// 				bootTime, _ := tc.event.BootTime()
// 				timeElapsed := time.Unix(0, tc.event.Birthdate).Sub(time.Unix(bootTime, 0)).Seconds()
// 				expectedBootHistogram.With(prometheus.Labels{hardwareLabel: hwVal, firmwareLabel: fwVal, rebootReasonLabel: rebootVal}).Observe(timeElapsed)
// 			}

// 			if !tc.expectedRebootDurationErr {
// 				timeElapsed := time.Unix(0, tc.event.Birthdate).Sub(time.Unix(0, tc.finderEvent.Birthdate)).Seconds()
// 				expectedRebootHistogram.With(prometheus.Labels{hardwareLabel: hwVal, firmwareLabel: fwVal, rebootReasonLabel: rebootVal}).Observe(timeElapsed)
// 			}

// 			testAssert := touchtest.New(t)
// 			testAssert.Expect(expectedRegistry)
// 			assert.True(testAssert.GatherAndCompare(actualRegistry))
// 		})
// 	}
// }

// func TestTimeBetweenEvents(t *testing.T) {
// 	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
// 	assert.Nil(t, err)
// 	finderErr := errors.New("finder error")
// 	tests := []struct {
// 		description         string
// 		incomingEvent       interpreter.Event
// 		startingEvent       interpreter.Event
// 		finderErr           error
// 		expectedTimeElapsed float64
// 		expectedErr         error
// 	}{
// 		{
// 			description:         "incoming event no birthdate",
// 			incomingEvent:       interpreter.Event{},
// 			startingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
// 			expectedTimeElapsed: -1,
// 			expectedErr:         errInvalidTimeElapsed,
// 		},
// 		{
// 			description:         "incoming event negative birthdate",
// 			incomingEvent:       interpreter.Event{Birthdate: -1000},
// 			startingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
// 			expectedTimeElapsed: -1,
// 			expectedErr:         errInvalidTimeElapsed,
// 		},
// 		{
// 			description:         "finder err",
// 			incomingEvent:       interpreter.Event{},
// 			startingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
// 			finderErr:           finderErr,
// 			expectedTimeElapsed: -1,
// 			expectedErr:         finderErr,
// 		},
// 		{
// 			description:         "starting event missing birthdate",
// 			incomingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
// 			startingEvent:       interpreter.Event{},
// 			expectedTimeElapsed: -1,
// 			expectedErr:         errInvalidTimeElapsed,
// 		},
// 		{
// 			description:         "starting event negative birthdate",
// 			incomingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
// 			startingEvent:       interpreter.Event{Birthdate: -1000},
// 			expectedTimeElapsed: -1,
// 			expectedErr:         errInvalidTimeElapsed,
// 		},
// 		{
// 			description:         "valid",
// 			incomingEvent:       interpreter.Event{Birthdate: now.Add(time.Minute).UnixNano()},
// 			startingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
// 			expectedTimeElapsed: now.Add(time.Minute).Sub(now).Seconds(),
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.description, func(t *testing.T) {
// 			assert := assert.New(t)
// 			testFinder := new(mockFinder)
// 			testFinder.On("Find", mock.Anything, mock.Anything).Return(tc.startingEvent, tc.finderErr)
// 			parser := RebootDurationParser{
// 				finder: testFinder,
// 				logger: zap.NewNop(),
// 			}
// 			timeElapsed, err := parser.timeBetweenEvents([]interpreter.Event{}, tc.incomingEvent)
// 			assert.Equal(tc.expectedErr, err)
// 			assert.Equal(tc.expectedTimeElapsed, timeElapsed)
// 		})
// 	}
// }
