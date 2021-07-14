package parsers

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"go.uber.org/zap"
)

func TestBootTimeCalculator(t *testing.T) {
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
			expectedErr:         errCalculation,
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
			expectedErr:         errCalculation,
		},
		{
			description: "no birthdate",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
			},
			expectedTimeElapsed: -1,
			expectedErr:         errCalculation,
		},
		{
			description: "neg birthdate",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				Birthdate: -1,
			},
			expectedTimeElapsed: -1,
			expectedErr:         errCalculation,
		},
		{
			description: "birthdate less than boot-time",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				Birthdate: now.Add(-2 * time.Minute).UnixNano(),
			},
			expectedTimeElapsed: -1,
			expectedErr:         errCalculation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			calculator := BootDurationCalculator(zap.NewNop(), func(_ interpreter.Event, duration float64) {
				assert.Equal(tc.expectedTimeElapsed, duration)
			})
			err := calculator.Calculate([]interpreter.Event{}, tc.event)
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestEventToCurrentCalculator(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	tests := []struct {
		description         string
		currentEvent        interpreter.Event
		finderEvent         interpreter.Event
		finderErr           error
		logger              *zap.Logger
		expectedTimeElapsed float64
		expectedErr         error
	}{
		{
			description: "success",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.Add(-1 * time.Minute).UnixNano(),
			},
			logger:              zap.NewNop(),
			expectedTimeElapsed: now.Sub(now.Add(-1 * time.Minute)).Seconds(),
		},
		{
			description: "nil logger success",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.Add(-1 * time.Minute).UnixNano(),
			},
			expectedTimeElapsed: now.Sub(now.Add(-1 * time.Minute)).Seconds(),
		},
		{
			description: "finder err",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			finderErr:   errors.New("test"),
			expectedErr: errEventNotFound,
		},
		{
			description: "current event missing birthdate",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.Add(-1 * time.Minute).UnixNano(),
			},
			expectedErr: errCalculation,
		},
		{
			description: "found event missing birthdate",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			expectedErr: errCalculation,
		},
		{
			description: "negative time elapsed",
			currentEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.Add(-1 * time.Minute).UnixNano(),
			},
			finderEvent: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Unix()),
				},
				Birthdate: now.UnixNano(),
			},
			expectedErr: errCalculation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			finder := new(mockFinder)
			finder.On("Find", mock.Anything, mock.Anything).Return(tc.finderEvent, tc.finderErr)
			calculator := EventToCurrentCalculator{
				logger:      tc.logger,
				eventFinder: finder,
				successCallback: func(_ interpreter.Event, _ interpreter.Event, duration float64) {
					assert.Equal(tc.expectedTimeElapsed, duration)
				},
			}
			err := calculator.Calculate([]interpreter.Event{}, tc.currentEvent)
			assert.Equal(tc.expectedErr, err)
		})
	}
}
