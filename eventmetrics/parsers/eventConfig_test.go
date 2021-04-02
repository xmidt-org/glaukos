package parsers

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
)

type testEvent struct {
	event         interpreter.Event
	expectedValid bool
}

func TestNewEventInfo(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	timeFunc := func() time.Time {
		return now
	}

	tests := []struct {
		description            string
		config                 EventConfig
		expectedCalculateUsing TimeLocation
		expectedErr            error
		testEvents             []testEvent
	}{
		{
			description: "valid-use birthdate",
			config: EventConfig{
				Regex:          ".*/online/.*",
				CalculateUsing: "Birthdate",
				ValidFrom:      -2 * time.Hour,
			},
			expectedCalculateUsing: Birthdate,
			testEvents: []testEvent{
				testEvent{
					event: interpreter.Event{
						Destination: "mac:112233445566/not-an-event/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-1 * time.Hour).UnixNano(),
					},
					expectedValid: false,
				},
				testEvent{
					event: interpreter.Event{
						Destination: "event:some-event/mac:112233445566/destination-mismatch/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-1 * time.Hour).UnixNano(),
					},
					expectedValid: false,
				},
				testEvent{
					event: interpreter.Event{
						Destination: "event:some-event/mac:112233445566/some-event/online/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-3 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-1 * time.Hour).UnixNano(),
					},
					expectedValid: false,
				},
				testEvent{
					event: interpreter.Event{
						Destination: "event:some-event/mac:112233445566/some-event/online/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-3 * time.Hour).UnixNano(),
					},
					expectedValid: false,
				},
				testEvent{
					event: interpreter.Event{
						Destination: "event:some-event/mac:112233445566/some-event/online/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-1 * time.Hour).UnixNano(),
					},
					expectedValid: true,
				},
			},
		},
		{
			description: "valid-use boot-time",
			config: EventConfig{
				Regex:          ".*/online/.*",
				CalculateUsing: "boot-time",
				ValidFrom:      -2 * time.Hour,
			},
			expectedCalculateUsing: Boottime,
			testEvents: []testEvent{
				testEvent{
					event: interpreter.Event{
						Destination: "mac:112233445566/not-an-event/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-1 * time.Hour).UnixNano(),
					},
					expectedValid: false,
				},
				testEvent{
					event: interpreter.Event{
						Destination: "event:some-event/mac:112233445566/destination-mismatch/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-1 * time.Hour).UnixNano(),
					},
					expectedValid: false,
				},
				testEvent{
					event: interpreter.Event{
						Destination: "event:some-event/mac:112233445566/online/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-3 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-1 * time.Hour).UnixNano(),
					},
					expectedValid: false,
				},
				testEvent{
					event: interpreter.Event{
						Destination: "event:some-event/mac:112233445566/online/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-3 * time.Hour).UnixNano(),
					},
					expectedValid: true,
				},
				testEvent{
					event: interpreter.Event{
						Destination: "event:some-event/mac:112233445566/online/random",
						Metadata: map[string]string{
							interpreter.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
						},
						Birthdate: now.Add(-1 * time.Hour).UnixNano(),
					},
					expectedValid: true,
				},
			},
		},
		{
			description: "invalid regex",
			config: EventConfig{
				Regex:          "[",
				CalculateUsing: "boot-time",
				ValidFrom:      -2 * time.Hour,
			},
			expectedErr: errInvalidRegex,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			event, err := NewEventInfo(tc.config, timeFunc)
			if tc.expectedErr == nil {
				assert.Nil(err)
				assert.Equal(tc.config.Regex, event.Regex.String())
				assert.Equal(tc.expectedCalculateUsing, event.CalculateUsing)
				for _, testEvent := range tc.testEvents {
					valid, _ := event.Valid(testEvent.event)
					assert.Equal(testEvent.expectedValid, valid)
				}
			} else {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			}

		})
	}
}

func TestValid(t *testing.T) {
	testError := errors.New("test error")
	val := new(mockValidator)
	val.On("Valid", mock.Anything).Return(false, testError)
	tests := []struct {
		description   string
		eventInfo     EventInfo
		expectedValid bool
		expectedErr   error
	}{
		{
			description: "Non-empty event info",
			eventInfo: EventInfo{
				Validator: val,
			},
			expectedValid: false,
			expectedErr:   testError,
		},
		{
			description:   "Empty Event",
			eventInfo:     EventInfo{},
			expectedValid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			valid, err := tc.eventInfo.Valid(interpreter.Event{})
			assert.Equal(tc.expectedValid, valid)
			assert.Equal(tc.expectedErr, err)
		})
	}
}
