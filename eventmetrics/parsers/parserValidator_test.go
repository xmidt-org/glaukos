package parsers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/validation"
	"github.com/xmidt-org/touchstone/touchtest"
	"go.uber.org/zap"
)

func TestValidate(t *testing.T) {
	testErr := errors.New("test")
	tests := []struct {
		description        string
		shouldActivate     func([]interpreter.Event, interpreter.Event) bool
		parserEvents       []interpreter.Event
		parserErr          error
		eventsValid        bool
		eventValidationErr error
		cycleValid         bool
		cycleValidationErr error
		expectedValid      bool
		expectedErr        error
	}{
		{
			description: "all valid",
			parserEvents: []interpreter.Event{
				interpreter.Event{},
			},
			eventsValid:   true,
			cycleValid:    true,
			expectedValid: true,
		},
		{
			description:    "don't activate",
			shouldActivate: func(_ []interpreter.Event, _ interpreter.Event) bool { return false },
			expectedValid:  true,
		},
		{
			description:   "parserErr",
			parserEvents:  []interpreter.Event{},
			parserErr:     testErr,
			expectedValid: false,
			expectedErr:   errFatal,
		},
		{
			description: "invalid event",
			parserEvents: []interpreter.Event{
				interpreter.Event{},
			},
			eventsValid:        false,
			eventValidationErr: testErr,
			cycleValid:         true,
			expectedValid:      false,
			expectedErr:        errValidation,
		},
		{
			description: "invalid cycle",
			parserEvents: []interpreter.Event{
				interpreter.Event{},
			},
			eventsValid:        true,
			cycleValid:         false,
			cycleValidationErr: testErr,
			expectedValid:      false,
			expectedErr:        errValidation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			mockParser := new(mockEventsParser)
			mockEventsValidator := new(mockValidator)
			mockCycleValidator := new(mockCycleValidator)

			mockParser.On("Parse", mock.Anything, mock.Anything).Return(tc.parserEvents, tc.parserErr)
			mockEventsValidator.On("Valid", mock.Anything).Return(tc.eventsValid, tc.eventValidationErr)
			mockCycleValidator.On("Valid", mock.Anything).Return(tc.cycleValid, tc.cycleValidationErr)

			eventCallbackCalled := false
			cycleCallbackCalled := false

			p := parserValidator{
				cycleParser:              mockParser,
				cycleValidator:           mockCycleValidator,
				eventsValidator:          mockEventsValidator,
				shouldActivate:           tc.shouldActivate,
				eventsValidationCallback: func(_ interpreter.Event, _ bool, _ error) { eventCallbackCalled = true },
				cycleValidationCallback:  func(_ bool, _ error) { cycleCallbackCalled = true },
			}

			valid, err := p.Validate([]interpreter.Event{}, interpreter.Event{})
			if (p.shouldActivate([]interpreter.Event{}, interpreter.Event{}) && tc.parserErr == nil) {
				assert.True(eventCallbackCalled)
				assert.True(cycleCallbackCalled)
			} else {
				assert.False(eventCallbackCalled)
				assert.False(cycleCallbackCalled)
			}

			assert.Equal(tc.expectedValid, valid)
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestSetDefaults(t *testing.T) {
	assert := assert.New(t)
	p := parserValidator{}
	p.setDefaults()
	assert.NotNil(p.shouldActivate)
	assert.NotNil(p.cycleParser)
	assert.NotNil(p.eventsValidator)
	assert.NotNil(p.cycleValidator)
	assert.NotNil(p.eventsValidationCallback)
	assert.NotNil(p.cycleValidationCallback)
}

func TestLogCycleError(t *testing.T) {
	tests := []struct {
		description  string
		err          error
		expectedTags []string
	}{
		{
			description:  "tagged Errs",
			expectedTags: []string{validation.RepeatedTransactionUUID.String(), validation.NonEvent.String()},
			err:          testTaggedErrors{tags: []validation.Tag{validation.RepeatedTransactionUUID, validation.NonEvent}},
		},
		{
			description:  "tagged Err",
			expectedTags: []string{validation.RepeatedTransactionUUID.String()},
			err:          testTaggedError{tag: validation.RepeatedTransactionUUID},
		},
		{
			description:  "other error",
			expectedTags: []string{validation.Unknown.String()},
			err:          errors.New("test"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				assert           = assert.New(t)
				expectedRegistry = prometheus.NewPedanticRegistry()
				expectedCounter  = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "cycleErrs",
						Help: "cycleErrs",
					},
					[]string{reasonLabel},
				)
				actualCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "cycleErrs",
						Help: "cycleErrs",
					},
					[]string{reasonLabel},
				)
			)

			expectedRegistry.Register(expectedCounter)
			logger := zap.NewNop()

			logCycleErr(tc.err, actualCounter, logger)
			for _, tag := range tc.expectedTags {
				expectedCounter.WithLabelValues(tag).Inc()
			}

			metricsAssert := touchtest.New(t)
			metricsAssert.Expect(expectedRegistry)
			assert.True(metricsAssert.CollectAndCompare(actualCounter))
		})
	}

}

func TestLogEventError(t *testing.T) {
	testEvent := interpreter.Event{
		Metadata: map[string]string{
			hardwareMetadataKey: "hw",
			firmwareMetadataKey: "fw",
		},
	}
	tests := []struct {
		description  string
		err          error
		expectedTags []string
	}{
		{
			description:  "tagged Errs",
			expectedTags: []string{validation.RepeatedTransactionUUID.String(), validation.NonEvent.String()},
			err:          testTaggedErrors{tags: []validation.Tag{validation.RepeatedTransactionUUID, validation.NonEvent}},
		},
		{
			description:  "tagged Err",
			expectedTags: []string{validation.RepeatedTransactionUUID.String()},
			err:          testTaggedError{tag: validation.RepeatedTransactionUUID},
		},
		{
			description:  "other error",
			expectedTags: []string{validation.Unknown.String()},
			err:          errors.New("test"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var (
				assert           = assert.New(t)
				expectedRegistry = prometheus.NewPedanticRegistry()
				expectedCounter  = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "eventErrs",
						Help: "eventErrs",
					},
					[]string{firmwareLabel, hardwareLabel, reasonLabel},
				)
				actualCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "eventErrs",
						Help: "eventErrs",
					},
					[]string{firmwareLabel, hardwareLabel, reasonLabel},
				)
			)

			expectedRegistry.Register(expectedCounter)
			logger := zap.NewNop()

			logEventError(logger, actualCounter, tc.err, testEvent)
			for _, tag := range tc.expectedTags {
				expectedCounter.WithLabelValues("fw", "hw", tag).Inc()
			}

			metricsAssert := touchtest.New(t)
			metricsAssert.Expect(expectedRegistry)
			assert.True(metricsAssert.CollectAndCompare(actualCounter))
		})
	}
}

func TestGetHardwareFirmware(t *testing.T) {
	tests := []struct {
		description   string
		event         interpreter.Event
		expectedHwVal string
		expectedFwVal string
		expectedFound bool
	}{
		{
			description: "all exists",
			event: interpreter.Event{
				Metadata: map[string]string{
					hardwareMetadataKey: "testHw",
					firmwareMetadataKey: "testFw",
				},
			},
			expectedHwVal: "testHw",
			expectedFwVal: "testFw",
			expectedFound: true,
		},
		{
			description: "missing hw",
			event: interpreter.Event{
				Metadata: map[string]string{
					firmwareMetadataKey: "testFw",
				},
			},
			expectedHwVal: unknownReason,
			expectedFwVal: "testFw",
			expectedFound: false,
		},
		{
			description: "missing fw",
			event: interpreter.Event{
				Metadata: map[string]string{
					hardwareMetadataKey: "testHw",
				},
			},
			expectedHwVal: "testHw",
			expectedFwVal: unknownReason,
			expectedFound: false,
		},
		{
			description: "missing both",
			event: interpreter.Event{
				Metadata: map[string]string{},
			},
			expectedHwVal: unknownReason,
			expectedFwVal: unknownReason,
			expectedFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			hwVal, fwVal, found := getHardwareFirmware(tc.event)
			assert.Equal(tc.expectedHwVal, hwVal)
			assert.Equal(tc.expectedFwVal, fwVal)
			assert.Equal(tc.expectedFound, found)
		})
	}
}

func TestTagsToString(t *testing.T) {
	tests := []struct {
		description    string
		tags           []validation.Tag
		expectedString string
	}{
		{
			description:    "multiple tags",
			tags:           []validation.Tag{validation.RepeatedTransactionUUID, validation.Unknown, validation.DuplicateEvent},
			expectedString: fmt.Sprintf("[%s, %s, %s]", validation.RepeatedTransactionUUIDStr, validation.UnknownStr, validation.DuplicateEventStr),
		},
		{
			description:    "one tag",
			tags:           []validation.Tag{validation.RepeatedTransactionUUID},
			expectedString: fmt.Sprintf("[%s]", validation.RepeatedTransactionUUIDStr),
		},
		{
			description:    "empty list",
			tags:           []validation.Tag{},
			expectedString: "[]",
		},
		{
			description:    "nil list",
			expectedString: "[]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			str := tagsToString(tc.tags)
			assert.Equal(tc.expectedString, str)
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
