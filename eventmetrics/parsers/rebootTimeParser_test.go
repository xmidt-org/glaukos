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
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/interpreter/validation"
	"github.com/xmidt-org/touchstone/touchtest"
	"go.uber.org/zap"
)

func TestRebootDurationParserName(t *testing.T) {
	name := "testRebootParser"
	parser := RebootDurationParser{
		name: name,
	}
	assert.Equal(t, name, parser.Name())
}

func TestParseValid(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	const (
		hwVal        = "hw"
		fwVal        = "fw"
		rebootReason = "reboot"
	)
	var (
		incomingBootTime    = now.Add(-5 * time.Minute)
		incomingBirthdate   = now
		startingBirthdate   = now.Add(-2 * time.Minute)
		validParser         = new(mockEventsParser)
		validCycleValidator = new(mockCycleValidator)
		validValidator      = new(mockValidator)
		finder              = new(mockFinder)
		client              = new(mockEventClient)
	)

	event := interpreter.Event{
		Destination: "event:device-status/mac:112233445566/fully-manageable",
		Metadata: map[string]string{
			interpreter.BootTimeKey: fmt.Sprint(incomingBootTime.Unix()),
			hardwareMetadataKey:     hwVal,
			firmwareMetadataKey:     fwVal,
			rebootReasonMetadataKey: rebootReason,
		},
		Birthdate: incomingBirthdate.UnixNano(),
	}

	validParser.On("Parse", mock.Anything, mock.Anything).Return([]interpreter.Event{interpreter.Event{}}, nil)
	validValidator.On("Valid", mock.Anything).Return(true, nil)
	validCycleValidator.On("Valid", mock.Anything).Return(true, nil)
	finder.On("Find", mock.Anything, mock.Anything).Return(interpreter.Event{Birthdate: startingBirthdate.UnixNano()}, nil)
	client.On("GetEvents", mock.Anything).Return([]interpreter.Event{})

	expectedBootToManageableHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "boot_to_manageable",
			Help:    "boot_to_manageable",
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		},
		[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
	)
	expectedRebootToManageableHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "reboot_to_manageable",
			Help:    "reboot_to_manageable",
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		},
		[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
	)

	m := Measures{
		BootToManageableHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "boot_to_manageable",
				Help:    "boot_to_manageable",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
			},
			[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
		),
		RebootToManageableHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "reboot_to_manageable",
				Help:    "reboot_to_manageable",
				Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
			},
			[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
		),
	}

	parser := RebootDurationParser{
		measures:         m,
		name:             "test_reboot_parser",
		logger:           zap.NewNop(),
		cycleParser:      validParser,
		validationParser: validParser,
		eventValidator:   validValidator,
		cycleValidator:   validCycleValidator,
		finder:           finder,
		client:           client,
	}

	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedBootToManageableHistogram)
	expectedRegistry.Register(expectedRebootToManageableHistogram)
	actualRegistry.Register(m.BootToManageableHistogram)
	actualRegistry.Register(m.RebootToManageableHistogram)

	assert := assert.New(t)
	parser.Parse(event)
	expectedBootToManageableHistogram.With(prometheus.Labels{hardwareLabel: hwVal, firmwareLabel: fwVal, rebootReasonLabel: rebootReason}).Observe(incomingBirthdate.Sub(incomingBootTime).Seconds())
	expectedRebootToManageableHistogram.With(prometheus.Labels{hardwareLabel: hwVal, firmwareLabel: fwVal, rebootReasonLabel: rebootReason}).Observe(incomingBirthdate.Sub(startingBirthdate).Seconds())
	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(testAssert.GatherAndCompare(actualRegistry))
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

func TestParseCalculationErr(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	const (
		hwVal = "hw"
		fwVal = "fw"
	)

	var (
		validParser         = new(mockEventsParser)
		validCycleValidator = new(mockCycleValidator)
		validValidator      = new(mockValidator)
		finder              = new(mockFinder)
		client              = new(mockEventClient)
	)

	event := interpreter.Event{
		Destination: "event:device-status/mac:112233445566/fully-manageable",
		Metadata: map[string]string{
			interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
			hardwareMetadataKey:     hwVal,
			firmwareMetadataKey:     fwVal,
		},
		Birthdate: now.Add(-2 * time.Minute).UnixNano(),
	}

	validParser.On("Parse", mock.Anything, mock.Anything).Return([]interpreter.Event{interpreter.Event{}}, nil)
	validValidator.On("Valid", mock.Anything).Return(true, nil)
	validCycleValidator.On("Valid", mock.Anything).Return(true, nil)
	finder.On("Find", mock.Anything, mock.Anything).Return(interpreter.Event{Birthdate: now.Add(2 * time.Minute).UnixNano()}, nil)
	client.On("GetEvents", mock.Anything).Return([]interpreter.Event{})

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

	parser := RebootDurationParser{
		measures:         m,
		name:             "test_reboot_parser",
		logger:           zap.NewNop(),
		cycleParser:      validParser,
		validationParser: validParser,
		eventValidator:   validValidator,
		cycleValidator:   validCycleValidator,
		finder:           finder,
		client:           client,
	}

	assert := assert.New(t)
	expectedRegistry := prometheus.NewPedanticRegistry()
	actualRegistry := prometheus.NewPedanticRegistry()
	expectedRegistry.Register(expectedTotalUnparsableCounter)
	expectedRegistry.Register(expectedRebootUnparsableCounter)
	actualRegistry.Register(m.TotalUnparsableEvents)
	actualRegistry.Register(m.RebootUnparsableCount)

	parser.Parse(event)

	expectedTotalUnparsableCounter.WithLabelValues("test_reboot_parser").Inc()
	expectedRebootUnparsableCounter.WithLabelValues(fwVal, hwVal, calculationErrReason).Inc()

	testAssert := touchtest.New(t)
	testAssert.Expect(expectedRegistry)
	assert.True(testAssert.GatherAndCompare(actualRegistry))
}

func TestParseValidationErr(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	hwVal := "hw"
	fwVal := "fw"
	validParser := new(mockEventsParser)
	invalidCycleValidator := new(mockCycleValidator)
	validCycleValidator := new(mockCycleValidator)
	invalidValidator := new(mockValidator)
	validValidator := new(mockValidator)
	client := new(mockEventClient)
	client.On("GetEvents", mock.Anything).Return([]interpreter.Event{})
	validParser.On("Parse", mock.Anything, mock.Anything).Return([]interpreter.Event{interpreter.Event{}}, nil)
	invalidValidator.On("Valid", mock.Anything).Return(false, errors.New("validation err"))
	validValidator.On("Valid", mock.Anything).Return(true, nil)
	invalidCycleValidator.On("Valid", mock.Anything).Return(false, errors.New("validation err"))
	validCycleValidator.On("Valid", mock.Anything).Return(true, nil)

	tests := []struct {
		description    string
		event          interpreter.Event
		eventValidator validation.Validator
		cycleValidator history.CycleValidator
	}{
		{
			description: "event validation error",
			event: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata: map[string]string{
					hardwareMetadataKey:     hwVal,
					firmwareMetadataKey:     fwVal,
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			eventValidator: invalidValidator,
			cycleValidator: validCycleValidator,
		},
		{
			description: "cycle validation error",
			event: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata: map[string]string{
					hardwareMetadataKey:     hwVal,
					firmwareMetadataKey:     fwVal,
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			eventValidator: validValidator,
			cycleValidator: invalidCycleValidator,
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
				RebootEventErrors: prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "reboot_event_errors",
						Help: "reboot_event_errors",
					},
					[]string{firmwareLabel, hardwareLabel, reasonLabel},
				),
				RebootCycleErrors: prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "reboot_cycle_errors",
						Help: "reboot_cycle_errors",
					},
					[]string{reasonLabel},
				),
			}

			parser := RebootDurationParser{
				measures:         m,
				name:             "test_reboot_parser",
				logger:           zap.NewNop(),
				cycleParser:      validParser,
				validationParser: validParser,
				eventValidator:   tc.eventValidator,
				cycleValidator:   tc.cycleValidator,
				client:           client,
			}

			expectedRegistry := prometheus.NewPedanticRegistry()
			actualRegistry := prometheus.NewPedanticRegistry()
			expectedRegistry.Register(expectedTotalUnparsableCounter)
			expectedRegistry.Register(expectedRebootUnparsableCounter)
			actualRegistry.Register(m.TotalUnparsableEvents)
			actualRegistry.Register(m.RebootUnparsableCount)

			expectedTotalUnparsableCounter.WithLabelValues("test_reboot_parser").Inc()
			expectedRebootUnparsableCounter.WithLabelValues(fwVal, hwVal, validationErrReason).Inc()

			parser.Parse(tc.event)
			testAssert := touchtest.New(t)
			testAssert.Expect(expectedRegistry)
			assert.True(testAssert.GatherAndCompare(actualRegistry))
		})
	}

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
		{
			description: "err parsing last boot-cycle",
			event: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/fully-manageable",
				Metadata: map[string]string{
					hardwareMetadataKey:     hwVal,
					firmwareMetadataKey:     fwVal,
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			cycleParser:      validParser,
			validationParser: invalidParser,
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
				measures:         m,
				name:             "test_reboot_parser",
				logger:           zap.NewNop(),
				cycleParser:      tc.cycleParser,
				validationParser: tc.validationParser,
				client:           client,
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
	eventsList := []interpreter.Event{
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-10 * time.Minute).Unix()),
			},
			Birthdate: now.Add(-5 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-20 * time.Minute).Unix()),
			},
			Birthdate: now.Add(-5 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-10 * time.Minute).Unix()),
			},
			Birthdate: now.Add(-4 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-5 * time.Minute).Unix()),
			},
		},
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-10 * time.Minute).Unix()),
			},
			Birthdate: now.Add(-3 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Metadata: map[string]string{
				interpreter.BootTimeKey: fmt.Sprint(now.Add(-5 * time.Minute).Unix()),
			},
			Birthdate: now.Add(-1 * time.Minute).UnixNano(),
		},
		interpreter.Event{
			Metadata:  map[string]string{},
			Birthdate: now.Add(-2 * time.Minute).UnixNano(),
		},
	}

	mockValidParser := new(mockEventsParser)
	mockInvalidParser := new(mockEventsParser)
	parsingErr := errors.New("parsing error")
	mockValidParser.On("Parse", mock.Anything, mock.Anything).Return(eventsList, nil)
	mockInvalidParser.On("Parse", mock.Anything, mock.Anything).Return([]interpreter.Event{}, parsingErr)
	expectedOrderedList := eventsList
	sort.Slice(expectedOrderedList, func(a, b int) bool {
		bootTimeA, _ := expectedOrderedList[a].BootTime()
		bootTimeB, _ := expectedOrderedList[b].BootTime()

		if bootTimeA != bootTimeB {
			return bootTimeA < bootTimeB
		}

		return expectedOrderedList[a].Birthdate < expectedOrderedList[b].Birthdate
	})

	tests := []struct {
		description  string
		event        interpreter.Event
		parser       EventsParser
		expectedList []interpreter.Event
		expectedErr  error
	}{
		{
			description:  "no device id",
			event:        interpreter.Event{},
			expectedList: []interpreter.Event{},
			expectedErr:  interpreter.ErrParseDeviceID,
		},
		{
			description: "error parsing",
			event: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/online",
			},
			parser:       mockInvalidParser,
			expectedList: []interpreter.Event{},
			expectedErr:  parsingErr,
		},
		{
			description: "valid parsing",
			event: interpreter.Event{
				Destination: "event:device-status/mac:112233445566/online",
			},
			parser:       mockValidParser,
			expectedList: expectedOrderedList,
		},
	}

	client := new(mockEventClient)
	client.On("GetEvents", mock.Anything).Return([]interpreter.Event{})

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			parser := RebootDurationParser{
				client:      client,
				logger:      zap.NewNop(),
				cycleParser: tc.parser,
			}

			events, err := parser.getEvents(tc.event)
			assert.Equal(tc.expectedErr, err)
			assert.Equal(tc.expectedList, events)
		})
	}
}

func createCycleValidators(errs []error, numValid int) []history.CycleValidator {
	cycleValidators := make([]history.CycleValidator, len(errs)+numValid)
	for i := 0; i < len(errs)+numValid; i++ {
		var validator *mockCycleValidator
		if i < len(errs) {
			validator = new(mockCycleValidator)
			validator.On("Valid", mock.Anything).Return(false, errs[i])
		} else {
			validator = new(mockCycleValidator)
			validator.On("Valid", mock.Anything).Return(true, nil)
		}
		cycleValidators[i] = validator
	}

	return cycleValidators
}

func TestCalculateDurations(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	hwVal := "hw"
	fwVal := "fw"
	rebootVal := "reboot"
	tests := []struct {
		description               string
		event                     interpreter.Event
		finderErr                 error
		finderEvent               interpreter.Event
		expectedBootDurationErr   bool
		expectedRebootDurationErr bool
		expectedValid             bool
	}{
		{
			description: "boot calculation err",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(time.Minute).Unix()),
					hardwareMetadataKey:     hwVal,
					firmwareMetadataKey:     fwVal,
					rebootReasonMetadataKey: rebootVal,
				},
				Birthdate: now.UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(time.Minute).Unix()),
				},
				Birthdate: now.Add(-2 * time.Minute).UnixNano(),
			},
			expectedBootDurationErr:   true,
			expectedRebootDurationErr: false,
			expectedValid:             false,
		},
		{
			description: "reboot calculation err",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
					hardwareMetadataKey:     hwVal,
					firmwareMetadataKey:     fwVal,
					rebootReasonMetadataKey: rebootVal,
				},
				Birthdate: now.Add(time.Minute).UnixNano(),
			},
			finderErr: errors.New("cannot find event"),
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(time.Minute).Unix()),
				},
				Birthdate: now.Add(-2 * time.Minute).UnixNano(),
			},
			expectedBootDurationErr:   false,
			expectedRebootDurationErr: true,
			expectedValid:             false,
		},
		{
			description: "all valid",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
					hardwareMetadataKey:     hwVal,
					firmwareMetadataKey:     fwVal,
					rebootReasonMetadataKey: rebootVal,
				},
				Birthdate: now.Add(time.Minute).UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(time.Minute).Unix()),
				},
				Birthdate: now.Add(-2 * time.Minute).UnixNano(),
			},
			expectedBootDurationErr:   false,
			expectedRebootDurationErr: false,
			expectedValid:             true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				assert                = assert.New(t)
				expectedRegistry      = prometheus.NewPedanticRegistry()
				actualRegistry        = prometheus.NewPedanticRegistry()
				expectedBootHistogram = prometheus.NewHistogramVec(
					prometheus.HistogramOpts{
						Name:    "boot_to_manageable",
						Help:    "boot_to_manageable",
						Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
					},
					[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
				)
				expectedRebootHistogram = prometheus.NewHistogramVec(
					prometheus.HistogramOpts{
						Name:    "reboot_to_manageable",
						Help:    "reboot_to_manageable",
						Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
					},
					[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
				)
				actualBootHistogram = prometheus.NewHistogramVec(
					prometheus.HistogramOpts{
						Name:    "boot_to_manageable",
						Help:    "boot_to_manageable",
						Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
					},
					[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
				)
				actualRebootHistogram = prometheus.NewHistogramVec(
					prometheus.HistogramOpts{
						Name:    "reboot_to_manageable",
						Help:    "reboot_to_manageable",
						Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
					},
					[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
				)
			)

			expectedRegistry.Register(expectedBootHistogram)
			expectedRegistry.Register(expectedRebootHistogram)
			actualRegistry.Register(actualBootHistogram)
			actualRegistry.Register(actualRebootHistogram)
			m := Measures{
				BootToManageableHistogram:   actualBootHistogram,
				RebootToManageableHistogram: actualRebootHistogram,
			}

			mockFinder := new(mockFinder)
			mockFinder.On("Find", mock.Anything, mock.Anything).Return(tc.finderEvent, tc.finderErr)
			parser := RebootDurationParser{
				logger:   zap.NewNop(),
				finder:   mockFinder,
				measures: m,
			}
			allValid := parser.calculateDurations([]interpreter.Event{}, tc.event)
			assert.Equal(tc.expectedValid, allValid)
			if !tc.expectedBootDurationErr {
				bootTime, _ := tc.event.BootTime()
				timeElapsed := time.Unix(0, tc.event.Birthdate).Sub(time.Unix(bootTime, 0)).Seconds()
				expectedBootHistogram.With(prometheus.Labels{hardwareLabel: hwVal, firmwareLabel: fwVal, rebootReasonLabel: rebootVal}).Observe(timeElapsed)
			}

			if !tc.expectedRebootDurationErr {
				timeElapsed := time.Unix(0, tc.event.Birthdate).Sub(time.Unix(0, tc.finderEvent.Birthdate)).Seconds()
				expectedRebootHistogram.With(prometheus.Labels{hardwareLabel: hwVal, firmwareLabel: fwVal, rebootReasonLabel: rebootVal}).Observe(timeElapsed)
			}

			testAssert := touchtest.New(t)
			testAssert.Expect(expectedRegistry)
			assert.True(testAssert.GatherAndCompare(actualRegistry))
		})
	}
}

func TestCalculateBootDuration(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	tests := []struct {
		description         string
		event               interpreter.Event
		expectedTimeElapsed float64
		expectedErr         error
	}{
		{
			description: "valid",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			expectedTimeElapsed: now.Sub(now.Add(-1 * time.Minute)).Seconds(),
		},
		{
			description: "no boot-time",
			event: interpreter.Event{
				Birthdate: now.UnixNano(),
			},
			expectedTimeElapsed: -1,
			expectedErr:         errInvalidTimeElapsed,
		},
		{
			description: "neg boot-time",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: "-1",
				},
				Birthdate: now.UnixNano(),
			},
			expectedTimeElapsed: -1,
			expectedErr:         errInvalidTimeElapsed,
		},
		{
			description: "no birthdate",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
			},
			expectedTimeElapsed: -1,
			expectedErr:         errInvalidTimeElapsed,
		},
		{
			description: "neg boot-time",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				Birthdate: -1,
			},
			expectedTimeElapsed: -1,
			expectedErr:         errInvalidTimeElapsed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			parser := RebootDurationParser{
				logger: zap.NewNop(),
			}
			timeElapsed, err := parser.calculateBootDuration(tc.event)
			assert.Equal(tc.expectedErr, err)
			assert.Equal(tc.expectedTimeElapsed, timeElapsed)
		})
	}
}

func TestTimeBetweenEvents(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	finderErr := errors.New("finder error")
	tests := []struct {
		description         string
		incomingEvent       interpreter.Event
		startingEvent       interpreter.Event
		finderErr           error
		expectedTimeElapsed float64
		expectedErr         error
	}{
		{
			description:         "incoming event no birthdate",
			incomingEvent:       interpreter.Event{},
			startingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
			expectedTimeElapsed: -1,
			expectedErr:         errInvalidTimeElapsed,
		},
		{
			description:         "incoming event negative birthdate",
			incomingEvent:       interpreter.Event{Birthdate: -1000},
			startingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
			expectedTimeElapsed: -1,
			expectedErr:         errInvalidTimeElapsed,
		},
		{
			description:         "finder err",
			incomingEvent:       interpreter.Event{},
			startingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
			finderErr:           finderErr,
			expectedTimeElapsed: -1,
			expectedErr:         finderErr,
		},
		{
			description:         "starting event missing birthdate",
			incomingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
			startingEvent:       interpreter.Event{},
			expectedTimeElapsed: -1,
			expectedErr:         errInvalidTimeElapsed,
		},
		{
			description:         "starting event negative birthdate",
			incomingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
			startingEvent:       interpreter.Event{Birthdate: -1000},
			expectedTimeElapsed: -1,
			expectedErr:         errInvalidTimeElapsed,
		},
		{
			description:         "valid",
			incomingEvent:       interpreter.Event{Birthdate: now.Add(time.Minute).UnixNano()},
			startingEvent:       interpreter.Event{Birthdate: now.UnixNano()},
			expectedTimeElapsed: now.Add(time.Minute).Sub(now).Seconds(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			testFinder := new(mockFinder)
			testFinder.On("Find", mock.Anything, mock.Anything).Return(tc.startingEvent, tc.finderErr)
			parser := RebootDurationParser{
				finder: testFinder,
				logger: zap.NewNop(),
			}
			timeElapsed, err := parser.timeBetweenEvents([]interpreter.Event{}, tc.incomingEvent)
			assert.Equal(tc.expectedErr, err)
			assert.Equal(tc.expectedTimeElapsed, timeElapsed)
		})
	}
}

func TestGetTimeElapsedHistogramLabels(t *testing.T) {
	tests := []struct {
		description    string
		event          interpreter.Event
		expectedLabels prometheus.Labels
	}{
		{
			description: "all exists",
			event: interpreter.Event{
				Metadata: map[string]string{
					hardwareMetadataKey:     "testHw",
					firmwareMetadataKey:     "testFw",
					rebootReasonMetadataKey: "testReboot",
				},
			},
			expectedLabels: prometheus.Labels{
				hardwareLabel:     "testHw",
				firmwareLabel:     "testFw",
				rebootReasonLabel: "testReboot",
			},
		},
		{
			description: "missing reboot reason",
			event: interpreter.Event{
				Metadata: map[string]string{
					hardwareMetadataKey: "testHw",
					firmwareMetadataKey: "testFw",
				},
			},
			expectedLabels: prometheus.Labels{
				hardwareLabel:     "testHw",
				firmwareLabel:     "testFw",
				rebootReasonLabel: unknownReason,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			labels := getTimeElapsedHistogramLabels(tc.event)
			assert.Equal(tc.expectedLabels, labels)
		})
	}
}
