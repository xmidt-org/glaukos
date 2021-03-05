package parsing

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEventValidator(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTime := func() time.Time {
		return now
	}

	tests := []struct {
		description       string
		rule              EventRule
		validator         TimeValidator
		expectedValidator EventValidator
		expectedErr       error
	}{
		{
			description: "Success with boot-time",
			rule: EventRule{
				Regex:            "/online/.*",
				CalculateUsing:   "Boot-time",
				DuplicateAllowed: true,
			},
			validator: TimeValidator{CurrentTime: currTime, ValidFrom: time.Hour, ValidTo: time.Hour},
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
			},
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
			expectedValidator: EventValidator{
				regex:            regexp.MustCompile("/online/.*"),
				calculateUsing:   Birthdate,
				duplicateAllowed: false,
			},
		},
		{
			description: "Unrecognized time location",
			rule: EventRule{
				Regex:          "/online/.*",
				CalculateUsing: "header",
			},
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
			expectedErr: errInvalidRegex,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			ev, err := NewEventValidator(tc.rule, tc.validator)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
				expected := tc.expectedValidator
				assert.Equal(expected.regex.String(), ev.regex.String())
				assert.NotNil(ev.timeValidator.CurrentTime)
				assert.Equal(expected.timeValidator.ValidFrom, ev.timeValidator.ValidFrom)
				assert.Equal(expected.timeValidator.ValidTo, ev.timeValidator.ValidTo)
				assert.Equal(expected.duplicateAllowed, ev.duplicateAllowed)
				expected.regex = ev.regex
				expected.timeValidator = ev.timeValidator
				assert.Equal(expected, ev)
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
	currTime := func() time.Time {
		return now
	}

	testRegex := regexp.MustCompile("device-status/.*/some-event/.*")
	testValidator := EventValidator{
		regex: testRegex,
		timeValidator: TimeValidator{
			CurrentTime: currTime,
			ValidFrom:   time.Hour,
			ValidTo:     time.Hour,
		},
	}
	tests := []struct {
		description string
		event       Event
		expectedRes bool
		expectedErr error
	}{
		{
			description: "Valid Event",
			event: Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/some-event/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
				BirthDate: now.UnixNano(),
			},
			expectedRes: true,
		},
		{
			description: "Wrong Event Type",
			event: Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/online",
			},
			expectedRes: false,
			expectedErr: errInvalidEventType,
		},
		{
			description: "No boot time",
			event: Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/some-event/1613033276/2s",
			},
			expectedRes: false,
			expectedErr: errPastDate,
		},
		{
			description: "Invalid boot time",
			event: Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/some-event/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Add(-200 * time.Hour).Unix()),
				},
			},
			expectedRes: false,
			expectedErr: errPastDate,
		},
		{
			description: "Invalid birthdate",
			event: Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/some-event/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
				BirthDate: now.Add(-200 * time.Hour).UnixNano(),
			},
			expectedRes: false,
			expectedErr: errPastDate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			res, err := testValidator.IsEventValid(tc.event)
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
