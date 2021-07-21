package parsers

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/bascule/basculechecks"
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

			cycleValidator := cycleValidation{
				validator: mockCycleValidator,
				parser:    mockParser,
				callback:  func(_ interpreter.Event, _ bool, _ error) { cycleCallbackCalled = true },
			}

			eventValidator := eventValidation{
				validator: mockEventsValidator,
				callback:  func(_ interpreter.Event, _ bool, _ error) { eventCallbackCalled = true },
			}
			p := NewParserValidator(cycleValidator, eventValidator, tc.shouldActivate)

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

func TestNewParserValidator(t *testing.T) {
	assert := assert.New(t)
	p := NewParserValidator(cycleValidation{}, eventValidation{}, nil)
	assert.NotNil(p.shouldActivate)
	assert.NotNil(p.cycleParser)
	assert.NotNil(p.eventValidator)
	assert.NotNil(p.cycleValidator)
	assert.NotNil(p.eventValidationCallback)
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
					[]string{reasonLabel, partnerIDLabel},
				)
				actualCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "cycleErrs",
						Help: "cycleErrs",
					},
					[]string{reasonLabel, partnerIDLabel},
				)
			)

			expectedRegistry.Register(expectedCounter)
			logger := zap.NewNop()

			logCycleErr(interpreter.Event{}, tc.err, actualCounter, logger)
			for _, tag := range tc.expectedTags {
				expectedCounter.WithLabelValues(tag, basculechecks.DeterminePartnerMetric([]string{})).Inc()
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
					[]string{firmwareLabel, hardwareLabel, reasonLabel, partnerIDLabel},
				)
				actualCounter = prometheus.NewCounterVec(
					prometheus.CounterOpts{
						Name: "eventErrs",
						Help: "eventErrs",
					},
					[]string{firmwareLabel, hardwareLabel, reasonLabel, partnerIDLabel},
				)
			)

			expectedRegistry.Register(expectedCounter)
			logger := zap.NewNop()

			logEventError(logger, actualCounter, tc.err, testEvent)
			for _, tag := range tc.expectedTags {
				expectedCounter.WithLabelValues("fw", "hw", tag, basculechecks.DeterminePartnerMetric(testEvent.PartnerIDs)).Inc()
			}

			metricsAssert := touchtest.New(t)
			metricsAssert.Expect(expectedRegistry)
			assert.True(metricsAssert.CollectAndCompare(actualCounter))
		})
	}
}
