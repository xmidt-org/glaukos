package validation

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/glaukos/message"
)

func TestBootTimeValidator(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTime := func() time.Time { return now }
	validation := TimeValidator{ValidFrom: -2 * time.Hour, ValidTo: time.Hour, Current: currTime}
	validator := BootTimeValidator(validation)
	tests := []struct {
		description string
		event       message.Event
		valid       bool
		expectedErr error
	}{
		{
			description: "Valid event",
			event: message.Event{
				Birthdate: now.Add(-1 * time.Hour).UnixNano(),
				Metadata: map[string]string{
					message.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
				},
			},
			valid: true,
		},
		{
			description: "Past boot-time",
			event: message.Event{
				Birthdate: now.Add(-1 * time.Hour).UnixNano(),
				Metadata: map[string]string{
					message.BootTimeKey: fmt.Sprint(now.Add(-3 * time.Hour).Unix()),
				},
			},
			valid:       false,
			expectedErr: InvalidEventErr{OriginalErr: InvalidBootTimeErr{OriginalErr: ErrPastDate}},
		},
		{
			description: "Future boot-time",
			event: message.Event{
				Birthdate: now.Add(-1 * time.Hour).UnixNano(),
				Metadata: map[string]string{
					message.BootTimeKey: fmt.Sprint(now.Add(2 * time.Hour).Unix()),
				},
			},
			valid:       false,
			expectedErr: InvalidEventErr{OriginalErr: InvalidBootTimeErr{OriginalErr: ErrFutureDate}},
		},
		{
			description: "No boot-time",
			event: message.Event{
				Birthdate: now.Add(-1 * time.Hour).UnixNano(),
				Metadata:  map[string]string{},
			},
			valid:       false,
			expectedErr: InvalidEventErr{OriginalErr: InvalidBootTimeErr{}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			valid, err := validator(tc.event)
			assert.Equal(tc.valid, valid)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.Contains(err.Error(), tc.expectedErr.Error())
			}
		})
	}
}

func TestBirthdateValidator(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTime := func() time.Time { return now }
	validation := TimeValidator{ValidFrom: -2 * time.Hour, ValidTo: time.Hour, Current: currTime}
	validator := BirthdateValidator(validation)
	tests := []struct {
		description string
		event       message.Event
		valid       bool
		expectedErr error
	}{
		{
			description: "Valid event",
			event: message.Event{
				Birthdate: now.Add(-1 * time.Hour).UnixNano(),
				Metadata: map[string]string{
					message.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
				},
			},
			valid: true,
		},
		{
			description: "Past birthdate",
			event: message.Event{
				Birthdate: now.Add(-3 * time.Hour).UnixNano(),
				Metadata: map[string]string{
					message.BootTimeKey: fmt.Sprint(now.Add(-1 * time.Hour).Unix()),
				},
			},
			valid:       false,
			expectedErr: InvalidEventErr{OriginalErr: InvalidBirthdateErr{OriginalErr: ErrPastDate}},
		},
		{
			description: "Future birthdate",
			event: message.Event{
				Birthdate: now.Add(2 * time.Hour).UnixNano(),
				Metadata: map[string]string{
					message.BootTimeKey: fmt.Sprint(now.Add(2 * time.Hour).Unix()),
				},
			},
			valid:       false,
			expectedErr: InvalidEventErr{OriginalErr: InvalidBirthdateErr{OriginalErr: ErrFutureDate}},
		},
		{
			description: "No birthdate",
			event: message.Event{
				Metadata: map[string]string{
					message.BootTimeKey: fmt.Sprint(now.Add(2 * time.Hour).Unix()),
				},
			},
			valid:       false,
			expectedErr: InvalidEventErr{OriginalErr: InvalidBirthdateErr{}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			valid, err := validator(tc.event)
			assert.Equal(tc.valid, valid)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.Contains(err.Error(), tc.expectedErr.Error())
			}
		})
	}
}

func TestDestinationValidator(t *testing.T) {
	validator := DestinationValidator(regexp.MustCompile(".*/some-event/.*"))
	tests := []struct {
		description string
		event       message.Event
		valid       bool
		expectedErr error
	}{
		{
			description: "Valid event",
			event: message.Event{
				Destination: "some-prefix/device-id/some-event/112233445566/random",
			},
			valid: true,
		},
		{
			description: "Invalid event",
			event: message.Event{
				Destination: "/random-event/",
			},
			expectedErr: InvalidEventErr{OriginalErr: ErrInvalidEventType},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			valid, err := validator(tc.event)
			assert.Equal(tc.valid, valid)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.Contains(err.Error(), tc.expectedErr.Error())
			}
		})
	}

}
