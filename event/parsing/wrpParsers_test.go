package parsing

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestGetWRPBootTime(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		description      string
		msg              wrp.Message
		expectedBootTime int64
		expectedErr      bool
	}{
		{
			description: "Success",
			msg: wrp.Message{
				Metadata: map[string]string{
					bootTimeKey: "1611700028",
				},
			},
			expectedBootTime: 1611700028,
		},
		{
			description: "No Boottime",
			msg: wrp.Message{
				Metadata: map[string]string{},
			},
			expectedBootTime: 0,
		},
		{
			description: "Int conversion error",
			msg: wrp.Message{
				Metadata: map[string]string{
					bootTimeKey: "not-a-number",
				},
			},
			expectedBootTime: 0,
			expectedErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			time, err := GetWRPBootTime(tc.msg)
			assert.Equal(tc.expectedBootTime, time)

			if tc.expectedErr {
				assert.NotNil(err)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestGetEventBootTime(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		description      string
		msg              Event
		expectedBootTime int64
		expectedErr      bool
	}{
		{
			description: "Success",
			msg: Event{
				Metadata: map[string]string{
					bootTimeKey: "1611700028",
				},
			},
			expectedBootTime: 1611700028,
		},
		{
			description: "No Boottime",
			msg: Event{
				Metadata: map[string]string{},
			},
			expectedBootTime: 0,
		},
		{
			description: "Int conversion error",
			msg: Event{
				Metadata: map[string]string{
					bootTimeKey: "not-a-number",
				},
			},
			expectedBootTime: 0,
			expectedErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			time, err := GetEventBootTime(tc.msg)
			assert.Equal(tc.expectedBootTime, time)

			if tc.expectedErr {
				assert.NotNil(err)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestGetDeviceID(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		description string
		destination string
		expectedErr error
		expectedID  string
	}{
		{
			description: "Success",
			destination: "event:device-status/mac:112233445566/offline",
			expectedID:  "mac:112233445566",
		},
		{
			description: "Invalid ID-missing event",
			destination: "mac:123",
			expectedErr: errors.New("error getting device ID from event"),
		},
		{
			description: "Invalid ID-missing event type",
			destination: "event:device-status/mac:123",
			expectedErr: errors.New("error getting device ID from event"),
		},
		{
			description: "Non device-status event",
			destination: "event:reboot/mac:123/offline",
			expectedID:  "mac:123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			deviceID, err := GetDeviceID(destinationRegex, tc.destination)
			assert.Equal(tc.expectedID, deviceID)
			assert.Equal(tc.expectedErr, err)
		})
	}
}
