package validation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEventValidators(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTime := func() time.Time {
		return now
	}
	tests := []struct {
		description string
		rule        RuleConfig
		length      int
		expectedErr error
	}{

		{
			description: "Success",
			rule: RuleConfig{
				Regex:          ".*/online/.*",
				CalculateUsing: "Birthdate",
				ValidFrom:      -1 * time.Hour,
			},
			length: 3,
		},
		{
			description: "No Birthdate",
			rule: RuleConfig{
				Regex:          ".*/online/.*",
				CalculateUsing: "Boot-time",
				ValidFrom:      -1 * time.Hour,
			},
			length: 2,
		},
		{
			description: "Invalid Regex",
			rule: RuleConfig{
				Regex:          `'(?=.*\d)'`,
				CalculateUsing: "Boot-time",
				ValidFrom:      -1 * time.Hour,
			},
			expectedErr: errInvalidRegex,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			validators, err := NewEventValidators(tc.rule, time.Hour, currTime)
			if tc.expectedErr == nil {
				assert.Equal(tc.length, len(validators))
				assert.Nil(err)
			} else {
				assert.Equal(tc.expectedErr, err)
				assert.Nil(validators)
			}
		})
	}
}

// func TestValidEvent(t *testing.T) {
// 	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
// 	assert.Nil(t, err)

// 	testRegex := regexp.MustCompile("device-status/.*/some-event/.*")

// 	tests := []struct {
// 		description  string
// 		event        message.Event
// 		timeIsValid  bool
// 		timeValError error
// 		timeLocation TimeLocation
// 		expectedRes  bool
// 		expectedErr  error
// 	}{
// 		{
// 			description: "Valid Event",
// 			event: message.Event{
// 				MsgType:     4,
// 				Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
// 				Metadata: map[string]string{
// 					message.BootTimeKey: fmt.Sprint(now.Unix()),
// 				},
// 				Birthdate: now.UnixNano(),
// 			},
// 			timeLocation: Birthdate,
// 			timeIsValid:  true,
// 			expectedRes:  true,
// 		},
// 		{
// 			description: "Wrong Event Type",
// 			event: message.Event{
// 				MsgType:     4,
// 				Destination: "event:device-status/mac:112233445566/online",
// 				Metadata: map[string]string{
// 					message.BootTimeKey: fmt.Sprint(now.Unix()),
// 				},
// 			},
// 			expectedRes: false,
// 			expectedErr: ErrInvalidEventType,
// 		},
// 		{
// 			description: "No boot-time",
// 			event: message.Event{
// 				Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
// 			},
// 			timeLocation: Boottime,
// 			expectedRes:  false,
// 			expectedErr:  ErrInvalidBootTime,
// 		},
// 		{
// 			description: "Boot-time Invalid",
// 			event: message.Event{
// 				Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
// 				Metadata:    map[string]string{message.BootTimeKey: "not-a-number"},
// 			},
// 			timeLocation: Boottime,
// 			expectedRes:  false,
// 			expectedErr:  ErrInvalidBootTime,
// 		},
// 		{
// 			description: "Boot-time Too Old",
// 			event: message.Event{
// 				Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
// 				Metadata:    map[string]string{message.BootTimeKey: "60"},
// 			},
// 			timeLocation: Boottime,
// 			timeIsValid:  false,
// 			timeValError: ErrPastDate,
// 			expectedRes:  false,
// 			expectedErr:  ErrInvalidBootTime,
// 		},
// 		{
// 			description: "Birthdate Invalid",
// 			event: message.Event{
// 				Destination: "event:device-status/mac:112233445566/some-event/1613033276/2s",
// 				Metadata: map[string]string{
// 					message.BootTimeKey: fmt.Sprint(now.Unix()),
// 				},
// 			},
// 			timeLocation: Birthdate,
// 			timeIsValid:  false,
// 			timeValError: ErrPastDate,
// 			expectedRes:  false,
// 			expectedErr:  ErrInvalidBirthdate,
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.description, func(t *testing.T) {
// 			assert := assert.New(t)
// 			mockTimeVal := new(mockTimeValidation)
// 			mockTimeVal.On("IsTimeValid", mock.Anything).Return(tc.timeIsValid, tc.timeValError).Once()
// 			validation := rule{
// 				regex:          testRegex,
// 				calculateUsing: tc.timeLocation,
// 				timeValidation: mockTimeVal,
// 			}
// 			res, err := validation.ValidateEvent(tc.event)
// 			if tc.expectedErr == nil || err == nil {
// 				assert.Equal(tc.expectedErr, err)
// 			} else {
// 				assert.True(errors.Is(err, tc.expectedErr),
// 					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
// 						err, tc.expectedErr),
// 				)
// 			}

// 			assert.Equal(tc.expectedRes, res)
// 		})
// 	}
// }

// func TestValidateType(t *testing.T) {
// 	testRegex := regexp.MustCompile("device-status/.*/some-event/.*")
// 	testVal := rule{
// 		regex: testRegex,
// 	}
// 	tests := []struct {
// 		destination string
// 		match       bool
// 	}{
// 		{
// 			destination: "device-status/some-random-string/some-event/129430124",
// 			match:       true,
// 		},
// 		{
// 			destination: "not-a-device-status-event",
// 			match:       false,
// 		},
// 		{
// 			destination: "device-status/some-string/online/123421",
// 			match:       false,
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.destination, func(t *testing.T) {
// 			assert := assert.New(t)
// 			match := testVal.ValidateType(tc.destination)
// 			assert.Equal(tc.match, match)
// 		})
// 	}
// }

// func TestGetEventCompareTime(t *testing.T) {
// 	tests := []struct {
// 		description  string
// 		event        message.Event
// 		timeLocation TimeLocation
// 		expectedTime time.Time
// 		expectedErr  error
// 	}{
// 		{
// 			description: "Successful with boot-time",
// 			event: message.Event{
// 				Metadata:  map[string]string{message.BootTimeKey: "50"},
// 				Birthdate: 60,
// 			},
// 			timeLocation: Boottime,
// 			expectedTime: time.Unix(50, 0),
// 		},
// 		{
// 			description: "Successful with birthdate",
// 			event: message.Event{
// 				Metadata:  map[string]string{message.BootTimeKey: "50"},
// 				Birthdate: 60,
// 			},
// 			timeLocation: Birthdate,
// 			expectedTime: time.Unix(0, 60),
// 		},
// 		{
// 			description: "Boot-time doesn't exist",
// 			event: message.Event{
// 				Metadata:  map[string]string{},
// 				Birthdate: 60,
// 			},
// 			timeLocation: Boottime,
// 			expectedTime: time.Time{},
// 			expectedErr:  message.ErrBootTimeNotFound,
// 		},
// 	}
// 	for _, tc := range tests {
// 		t.Run(tc.description, func(t *testing.T) {
// 			assert := assert.New(t)
// 			validation := rule{calculateUsing: tc.timeLocation}
// 			time, err := validation.GetCompareTime(tc.event)
// 			assert.True(tc.expectedTime.Equal(time))
// 			assert.Equal(tc.expectedErr, err)
// 		})
// 	}
// }
