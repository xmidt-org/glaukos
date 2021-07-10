package parsers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/validation"
	"github.com/xmidt-org/touchstone/touchtest"
	"go.uber.org/zap"
)

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
			m := Measures{
				RebootEventErrors: actualCounter,
			}

			parser := RebootDurationParser{
				measures: m,
				name:     "test_reboot_parser",
				logger:   zap.NewNop(),
			}

			logger := zap.NewNop()

			logEventError(tc.err, testEvent, "123")
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
