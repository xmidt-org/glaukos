package parsing

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestNewEventValidation(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTime := func() time.Time {
		return now
	}

	tests := []struct {
		description string
		rule        EventRule
		validDest   string
		validDate   time.Time
		invalidDest string
		invalidDate time.Time
		expectedErr error
	}{
		{
			description: "Success with boot-time",
			rule: EventRule{
				Regex:          ".*/online/.*",
				CalculateUsing: "Boot-time",
				ValidFrom:      -1 * time.Hour,
			},
			validDest:   "whatever/online/hello",
			invalidDest: "random",
			validDate:   now.Add(-30 * time.Minute),
			invalidDate: now.Add(-2 * time.Hour),
		},
		{
			description: "Success with birthdate",
			rule: EventRule{
				Regex:          ".*/online/.*",
				CalculateUsing: "Birthdate",
				ValidFrom:      -1 * time.Hour,
			},
			validDest:   "whatever/online/hello",
			invalidDest: "random",
			validDate:   now.Add(-30 * time.Minute),
			invalidDate: now.Add(-2 * time.Hour),
		},
		{
			description: "Success with defaults",
			rule: EventRule{
				Regex: ".*/online/.*",
			},
			validDest:   "whatever/online/hello",
			invalidDest: "random",
			validDate:   now.Add(-30 * time.Minute),
			invalidDate: now.Add(-2 * time.Hour),
		},
		{
			description: "Unrecognized time location",
			rule: EventRule{
				Regex:          ".*/online/.*",
				CalculateUsing: "header",
				ValidFrom:      -1 * time.Hour,
			},
			validDest:   "whatever/online/hello",
			invalidDest: "random",
			validDate:   now.Add(-30 * time.Minute),
			invalidDate: now.Add(-2 * time.Hour),
		},
		{
			description: "regex error",
			rule: EventRule{
				Regex: `'(?=.*\d)'`,
			},
			expectedErr: ErrInvalidRegex,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			ev, err := NewEventValidation(tc.rule, time.Hour, currTime)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
				assert.NotNil(ev)
			} else {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
				assert.Empty(ev)
			}
		})
	}
}

func TestIsEventValid(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	testRegex := regexp.MustCompile("device-status/.*/some-event/.*")

	tests := []struct {
		description  string
		event        client.Event
		timeIsValid  bool
		timeValError error
		timeLocation TimeLocation
		expectedRes  bool
		expectedErr  error
	}{
		{
			description: "Valid Event",
			event: client.Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/some-event/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
				BirthDate: now.UnixNano(),
			},
			timeLocation: Birthdate,
			timeIsValid:  true,
			expectedRes:  true,
		},
		{
			description: "Wrong Event Type",
			event: client.Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			expectedRes: false,
			expectedErr: ErrInvalidEventType,
		},
		{
			description: "No boot-time",
			event: client.Event{
				Dest: "event:device-status/mac:112233445566/some-event/1613033276/2s",
			},
			timeLocation: Boottime,
			expectedRes:  false,
			expectedErr:  ErrInvalidBootTime,
		},
		{
			description: "Boot-time Invalid",
			event: client.Event{
				Dest:     "event:device-status/mac:112233445566/some-event/1613033276/2s",
				Metadata: map[string]string{bootTimeKey: "not-a-number"},
			},
			timeLocation: Boottime,
			expectedRes:  false,
			expectedErr:  ErrInvalidBootTime,
		},
		{
			description: "Boot-time Too Old",
			event: client.Event{
				Dest:     "event:device-status/mac:112233445566/some-event/1613033276/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			timeLocation: Boottime,
			timeIsValid:  false,
			timeValError: ErrPastDate,
			expectedRes:  false,
			expectedErr:  ErrInvalidBootTime,
		},
		{
			description: "Birthdate Invalid",
			event: client.Event{
				Dest: "event:device-status/mac:112233445566/some-event/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			timeLocation: Birthdate,
			timeIsValid:  false,
			timeValError: ErrPastDate,
			expectedRes:  false,
			expectedErr:  ErrInvalidBirthdate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			mockTimeVal := new(MockTimeValidation)
			mockTimeVal.On("IsTimeValid", mock.Anything).Return(tc.timeIsValid, tc.timeValError).Once()
			ev := eventValidator{
				regex:          testRegex,
				calculateUsing: tc.timeLocation,
				timeValidation: mockTimeVal,
			}
			res, err := ev.IsEventValid(tc.event)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			}

			assert.Equal(tc.expectedRes, res)
		})
	}
}

func TestIsWRPValid(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	testRegex := regexp.MustCompile("device-status/.*/some-event/.*")

	tests := []struct {
		description  string
		wrp          queue.WrpWithTime
		timeIsValid  bool
		timeValError error
		timeLocation TimeLocation
		expectedRes  bool
		expectedErr  error
	}{
		{
			description: "Valid WRP",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Unix()),
					},
				},
				Beginning: now,
			},
			timeLocation: Birthdate,
			timeIsValid:  true,
			expectedRes:  true,
		},
		{
			description: "Wrong Event Type",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/online",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Unix()),
					},
				},
			},
			expectedRes: false,
			expectedErr: ErrInvalidEventType,
		},
		{
			description: "No Boot-time",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
					Metadata:    map[string]string{},
				},
			},
			timeLocation: Boottime,
			expectedRes:  false,
			expectedErr:  ErrInvalidBootTime,
		},
		{
			description: "Boot-time Invalid",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
					Metadata:    map[string]string{bootTimeKey: "not-a-number"},
				},
			},
			timeLocation: Boottime,
			expectedRes:  false,
			expectedErr:  ErrInvalidBootTime,
		},
		{
			description: "Boot-time Too Old",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
					Metadata:    map[string]string{bootTimeKey: "60"},
				},
			},
			timeLocation: Boottime,
			timeIsValid:  false,
			timeValError: ErrPastDate,
			expectedRes:  false,
			expectedErr:  ErrInvalidBootTime,
		},
		{
			description: "Birthdate Invalid",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Unix()),
					},
				},
			},
			timeLocation: Birthdate,
			timeIsValid:  false,
			timeValError: ErrPastDate,
			expectedRes:  false,
			expectedErr:  ErrInvalidBirthdate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			mockTimeVal := new(MockTimeValidation)
			mockTimeVal.On("IsTimeValid", mock.Anything).Return(tc.timeIsValid, tc.timeValError).Once()
			ev := eventValidator{
				regex:          testRegex,
				calculateUsing: tc.timeLocation,
				timeValidation: mockTimeVal,
			}
			res, err := ev.IsWRPValid(tc.wrp)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			}

			assert.Equal(tc.expectedRes, res)
		})
	}
}

func TestValidateType(t *testing.T) {
	testRegex := regexp.MustCompile("device-status/.*/some-event/.*")
	testVal := eventValidator{
		regex: testRegex,
	}
	tests := []struct {
		destination string
		match       bool
	}{
		{
			destination: "device-status/some-random-string/some-event/129430124",
			match:       true,
		},
		{
			destination: "not-a-device-status-event",
			match:       false,
		},
		{
			destination: "device-status/some-string/online/123421",
			match:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.destination, func(t *testing.T) {
			assert := assert.New(t)
			match := testVal.ValidateType(tc.destination)
			assert.Equal(tc.match, match)
		})
	}
}

func TestGetEventCompareTime(t *testing.T) {
	tests := []struct {
		description  string
		event        client.Event
		timeLocation TimeLocation
		expectedTime time.Time
		expectedErr  error
	}{
		{
			description: "Successful with boot-time",
			event: client.Event{
				Metadata:  map[string]string{bootTimeKey: "50"},
				BirthDate: 60,
			},
			timeLocation: Boottime,
			expectedTime: time.Unix(50, 0),
		},
		{
			description: "Successful with birthdate",
			event: client.Event{
				Metadata:  map[string]string{bootTimeKey: "50"},
				BirthDate: 60,
			},
			timeLocation: Birthdate,
			expectedTime: time.Unix(0, 60),
		},
		{
			description: "Boot-time doesn't exist",
			event: client.Event{
				Metadata:  map[string]string{},
				BirthDate: 60,
			},
			timeLocation: Boottime,
			expectedTime: time.Time{},
			expectedErr:  ErrBootTimeNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			ev := eventValidator{calculateUsing: tc.timeLocation}
			time, err := ev.GetEventCompareTime(tc.event)
			assert.True(tc.expectedTime.Equal(time))
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestGetWRPCompareTime(t *testing.T) {
	tests := []struct {
		description  string
		wrp          queue.WrpWithTime
		timeLocation TimeLocation
		expectedTime time.Time
		expectedErr  error
	}{
		{
			description: "Successful with boot-time",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Metadata: map[string]string{bootTimeKey: "50"},
				},
				Beginning: time.Unix(0, 60),
			},
			timeLocation: Boottime,
			expectedTime: time.Unix(50, 0),
		},
		{
			description: "Successful with birthdate",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Metadata: map[string]string{bootTimeKey: "50"},
				},
				Beginning: time.Unix(0, 60),
			},
			timeLocation: Birthdate,
			expectedTime: time.Unix(0, 60),
		},
		{
			description: "Boot-time doesn't exist",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Metadata: map[string]string{},
				},
				Beginning: time.Unix(0, 60),
			},
			timeLocation: Boottime,
			expectedTime: time.Time{},
			expectedErr:  ErrBootTimeNotFound,
		},
		{
			description: "Default Birthdate",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Metadata: map[string]string{},
				},
			},
			timeLocation: Birthdate,
			expectedTime: time.Time{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			ev := eventValidator{calculateUsing: tc.timeLocation}
			time, err := ev.GetWRPCompareTime(tc.wrp)
			assert.True(tc.expectedTime.Equal(time))
			assert.Equal(tc.expectedErr, err)
		})
	}
}
