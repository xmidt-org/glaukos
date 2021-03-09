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

func TestNewEventValidator(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTime := func() time.Time {
		return now
	}
	defaultValidator := TimeValidator{
		Current:   currTime,
		ValidFrom: -1 * time.Hour,
		ValidTo:   time.Hour,
	}

	tests := []struct {
		description       string
		rule              EventRule
		defaultValidator  TimeValidation
		expectedValidator EventValidator
		expectedErr       error
	}{
		{
			description: "Success with boot-time",
			rule: EventRule{
				Regex:            "/online/.*",
				CalculateUsing:   "Boot-time",
				DuplicateAllowed: true,
				ValidFrom:        -1 * time.Hour,
			},
			defaultValidator: defaultValidator,
			expectedValidator: EventValidator{
				regex:            regexp.MustCompile("/online/.*"),
				calculateUsing:   Boottime,
				duplicateAllowed: true,
			},
		},
		{
			description: "Success with birthdate",
			rule: EventRule{
				Regex:            "/online/.*",
				CalculateUsing:   "Birthdate",
				DuplicateAllowed: true,
				ValidFrom:        -1 * time.Hour,
			},
			defaultValidator: defaultValidator,
			expectedValidator: EventValidator{
				regex:            regexp.MustCompile("/online/.*"),
				calculateUsing:   Birthdate,
				duplicateAllowed: true,
			},
		},
		{
			description: "Success with defaults",
			rule: EventRule{
				Regex: "/online/.*",
			},
			defaultValidator: defaultValidator,
			expectedValidator: EventValidator{
				regex:            regexp.MustCompile("/online/.*"),
				calculateUsing:   Birthdate,
				duplicateAllowed: false,
				timeValidation:   defaultValidator,
			},
		},
		{
			description: "Unrecognized time location",
			rule: EventRule{
				Regex:          "/online/.*",
				CalculateUsing: "header",
				ValidFrom:      -1 * time.Hour,
			},
			defaultValidator: defaultValidator,
			expectedValidator: EventValidator{
				regex:            regexp.MustCompile("/online/.*"),
				calculateUsing:   Birthdate,
				duplicateAllowed: false,
			},
		},
		{
			description: "regex error",
			rule: EventRule{
				Regex: `'(?=.*\d)'`,
			},
			defaultValidator: defaultValidator,
			expectedErr:      errInvalidRegex,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			ev, err := NewEventValidator(tc.rule, tc.defaultValidator)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
				expected := tc.expectedValidator
				assert.Equal(expected.regex.String(), ev.regex.String())
				assert.NotEmpty(ev.timeValidation)
				assert.Equal(expected.duplicateAllowed, ev.duplicateAllowed)
				assert.Equal(expected.calculateUsing, ev.calculateUsing)
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
			},
			expectedRes: false,
			expectedErr: errInvalidEventType,
		},
		{
			description: "No boot-time",
			event: client.Event{
				Dest: "event:device-status/mac:112233445566/some-event/1613033276/2s",
			},
			timeLocation: Boottime,
			expectedRes:  false,
			expectedErr:  errBootTimeNotFound,
		},
		{
			description: "Boot-time Invalid",
			event: client.Event{
				Dest:     "event:device-status/mac:112233445566/some-event/1613033276/2s",
				Metadata: map[string]string{bootTimeKey: "not-a-number"},
			},
			timeLocation: Boottime,
			expectedRes:  false,
			expectedErr:  errBootTimeParse,
		},
		{
			description: "Boot-time Too Old",
			event: client.Event{
				Dest:     "event:device-status/mac:112233445566/some-event/1613033276/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			timeLocation: Boottime,
			timeIsValid:  false,
			timeValError: errPastDate,
			expectedRes:  false,
			expectedErr:  errPastDate,
		},
		{
			description: "Birthdate Invalid",
			event: client.Event{
				Dest: "event:device-status/mac:112233445566/some-event/1613033276/2s",
			},
			timeLocation: Birthdate,
			timeIsValid:  false,
			timeValError: errPastDate,
			expectedRes:  false,
			expectedErr:  errPastDate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			mockTimeVal := new(MockTimeValidation)
			mockTimeVal.On("IsTimeValid", mock.Anything).Return(tc.timeIsValid, tc.timeValError).Once()
			ev := EventValidator{
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
				},
			},
			expectedRes: false,
			expectedErr: errInvalidEventType,
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
			expectedErr:  errBootTimeNotFound,
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
			expectedErr:  errBootTimeParse,
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
			timeValError: errPastDate,
			expectedRes:  false,
			expectedErr:  errPastDate,
		},
		{
			description: "Boot-time Invalid",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
				},
			},
			timeLocation: Birthdate,
			timeIsValid:  false,
			timeValError: errPastDate,
			expectedRes:  false,
			expectedErr:  errPastDate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			mockTimeVal := new(MockTimeValidation)
			mockTimeVal.On("IsTimeValid", mock.Anything).Return(tc.timeIsValid, tc.timeValError).Once()
			ev := EventValidator{
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
	testVal := EventValidator{
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
			expectedErr:  errBootTimeNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			ev := EventValidator{calculateUsing: tc.timeLocation}
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
			expectedErr:  errBootTimeNotFound,
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
			ev := EventValidator{calculateUsing: tc.timeLocation}
			time, err := ev.GetWRPCompareTime(tc.wrp)
			assert.True(tc.expectedTime.Equal(time))
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestDuplicateAllowed(t *testing.T) {
	ev := EventValidator{duplicateAllowed: true}
	assert.True(t, ev.DuplicateAllowed())
}
