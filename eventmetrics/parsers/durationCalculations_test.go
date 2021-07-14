package parsers

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
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
			description: "neg birthdate",
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				Birthdate: -1,
			},
			expectedTimeElapsed: -1,
			expectedErr:         errInvalidTimeElapsed,
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
			expectedErr:         errInvalidTimeElapsed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			hist := prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "testHistogram",
					Help:    "testHistogram",
					Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
				},
				[]string{firmwareLabel, hardwareLabel, rebootReasonLabel},
			)
			assert := assert.New(t)
			calculator := BootDurationCalculator(zap.NewNop(), hist)
			err := calculator.Calculate([]interpreter.Event{}, tc.event)
			assert.Equal(tc.expectedErr, err)
		})
	}
}

// func TestCalculateBootDuration(t *testing.T) {

// }
