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

func TestParseTimeLocation(t *testing.T) {
	tests := []struct {
		testLocation     string
		expectedLocation TimeLocation
	}{
		{
			testLocation:     "Birthdate",
			expectedLocation: Birthdate,
		},
		{
			testLocation:     "Boot-time",
			expectedLocation: Boottime,
		},
		{
			testLocation:     "birthdate",
			expectedLocation: Birthdate,
		},
		{
			testLocation:     "boot-time",
			expectedLocation: Boottime,
		},
		{
			testLocation:     "random",
			expectedLocation: Birthdate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testLocation, func(t *testing.T) {
			assert := assert.New(t)
			res := ParseTimeLocation(tc.testLocation)
			assert.Equal(tc.expectedLocation, res)
		})
	}
}

func TestParseTime(t *testing.T) {
	birthdate, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	bootTime, err := time.Parse(time.RFC3339Nano, "2021-03-01T18:00:01Z")
	assert.Nil(t, err)

	tests := []struct {
		description  string
		location     TimeLocation
		expectedTime time.Time
		event        interpreter.Event
	}{
		{
			description:  "Valid Birthdate",
			location:     Birthdate,
			expectedTime: birthdate,
			event: interpreter.Event{
				Birthdate: birthdate.UnixNano(),
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(bootTime.Unix()),
				},
			},
		},
		{
			description:  "Valid Boot-time",
			location:     Boottime,
			expectedTime: bootTime,
			event: interpreter.Event{
				Birthdate: birthdate.UnixNano(),
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(bootTime.Unix()),
				},
			},
		},
		{
			description:  "Invalid Birthdate",
			location:     Birthdate,
			expectedTime: time.Time{},
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(bootTime.Unix()),
				},
			},
		},
		{
			description:  "Invalid Boot-time",
			location:     Boottime,
			expectedTime: time.Time{},
			event: interpreter.Event{
				Birthdate: birthdate.UnixNano(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			time := ParseTime(tc.event, tc.location)
			assert.True(tc.expectedTime.Equal(time))
			assert.Nil(err)
		})
	}
}

func TestParseSessionType(t *testing.T) {
	tests := []struct {
		testStr      string
		expectedType SessionType
	}{
		{
			testStr:      "Previous",
			expectedType: Previous,
		},
		{
			testStr:      "Current",
			expectedType: Current,
		},
		{
			testStr:      "previous",
			expectedType: Previous,
		},
		{
			testStr:      "current",
			expectedType: Current,
		},
		{
			testStr:      "random",
			expectedType: Previous,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testStr, func(t *testing.T) {
			assert := assert.New(t)
			res := ParseSessionType(tc.testStr)
			assert.Equal(tc.expectedType, res)
		})
	}
}

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
