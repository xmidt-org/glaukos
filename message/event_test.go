package message

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/xmidt-org/wrp-go/v3"

	"github.com/stretchr/testify/assert"
)

func TestNewEvent(t *testing.T) {
	timeString := "2021-03-02T18:00:01Z"
	now, err := time.Parse(time.RFC3339Nano, timeString)
	assert.Nil(t, err)

	tests := []struct {
		description string
		msg         wrp.Message
		expected    Event
		expectedErr error
	}{
		{
			description: "birthdate in payload",
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Source:          "test-source",
				Destination:     "test-destination",
				TransactionUUID: "some-ID",
				Metadata:        map[string]string{"key1": "value1", "key2": "value2"},
				Payload:         []byte(fmt.Sprintf(`{"ts":"%s"}`, timeString)),
			},
			expected: Event{
				MsgType:         int(wrp.SimpleEventMessageType),
				Source:          "test-source",
				Destination:     "test-destination",
				TransactionUUID: "some-ID",
				Metadata:        map[string]string{"key1": "value1", "key2": "value2"},
				Payload:         fmt.Sprintf(`{"ts":"%s"}`, timeString),
				Birthdate:       now.UnixNano(),
			},
		},
		{
			description: "no birthdate in payload",
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Source:          "test-source",
				Destination:     "test-destination",
				TransactionUUID: "some-ID",
				Metadata:        map[string]string{"key1": "value1", "key2": "value2"},
				Payload:         []byte(`{"random":"some-value"`),
			},
			expected: Event{
				MsgType:         int(wrp.SimpleEventMessageType),
				Source:          "test-source",
				Destination:     "test-destination",
				TransactionUUID: "some-ID",
				Metadata:        map[string]string{"key1": "value1", "key2": "value2"},
				Payload:         `{"random":"some-value"`,
			},
			expectedErr: ErrBirthdateParse,
		},
		{
			description: "no payload, no metadata",
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Source:          "test-source",
				Destination:     "test-destination",
				TransactionUUID: "some-ID",
			},
			expected: Event{
				MsgType:         int(wrp.SimpleEventMessageType),
				Source:          "test-source",
				Destination:     "test-destination",
				TransactionUUID: "some-ID",
			},
			expectedErr: ErrBirthdateParse,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			event, err := NewEvent(tc.msg)
			assert.Equal(tc.expected, event)
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestBootTime(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		description      string
		msg              Event
		expectedBootTime int64
		expectedErr      error
	}{
		{
			description: "Success",
			msg: Event{
				Metadata: map[string]string{
					BootTimeKey: "1611700028",
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
			expectedErr:      ErrBootTimeNotFound,
		},
		{
			description:      "No Metadata",
			msg:              Event{},
			expectedBootTime: 0,
			expectedErr:      ErrBootTimeNotFound,
		},
		{
			description: "Key with slash",
			msg: Event{
				Metadata: map[string]string{
					"/boot-time": "1000",
				},
			},
			expectedBootTime: 1000,
		},
		{
			description: "Key without slash",
			msg: Event{
				Metadata: map[string]string{
					"boot-time": "1000",
				},
			},
			expectedBootTime: 1000,
		},
		{
			description: "Int conversion error",
			msg: Event{
				Metadata: map[string]string{
					BootTimeKey: "not-a-number",
				},
			},
			expectedBootTime: 0,
			expectedErr:      ErrBootTimeParse,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			time, err := tc.msg.BootTime()
			assert.Equal(tc.expectedBootTime, time)

			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			}
		})
	}
}

func TestGetDeviceID(t *testing.T) {
	destinationRegex := regexp.MustCompile(`^(?P<event>[^/]+)/((?P<prefix>(?i)mac|uuid|dns|serial):(?P<id>[^/]+))/(?P<type>[^/\s]+)`)
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
			expectedErr: ErrParseDeviceID,
		},
		{
			description: "Invalid ID-missing event type",
			destination: "event:device-status/mac:123",
			expectedErr: ErrParseDeviceID,
		},
		{
			description: "Non device-status event",
			destination: "event:reboot/mac:123/offline",
			expectedID:  "mac:123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			e := Event{Destination: tc.destination}
			deviceID, err := e.DeviceID(destinationRegex)
			assert.Equal(tc.expectedID, deviceID)
			if err != nil || tc.expectedErr != nil {
				assert.True(errors.Is(err, tc.expectedErr))
			}

		})
	}
}

func TestGetMetadataValue(t *testing.T) {
	tests := []struct {
		description string
		metadata    map[string]string
		key         string
		expectedVal string
		expectedOk  bool
	}{
		{
			description: "Key exists",
			metadata:    map[string]string{"/key": "val"},
			key:         "/key",
			expectedVal: "val",
			expectedOk:  true,
		},
		{
			description: "Key exists, without slash",
			metadata:    map[string]string{"key": "val"},
			key:         "/key",
			expectedVal: "val",
			expectedOk:  true,
		},
		{
			description: "Key doesn't exist",
			metadata:    map[string]string{"test": "val"},
			key:         "/key",
			expectedVal: "",
			expectedOk:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			e := Event{Metadata: tc.metadata}
			val, ok := e.GetMetadataValue(tc.key)
			assert.Equal(tc.expectedVal, val)
			assert.Equal(tc.expectedOk, ok)
		})
	}
}

func TestGetBirthDate(t *testing.T) {
	goodTime, err := time.Parse(time.RFC3339Nano, "2019-02-13T21:19:02.614191735Z")
	assert.Nil(t, err)
	tests := []struct {
		description   string
		payload       []byte
		expectedTime  time.Time
		expectedFound bool
	}{
		{
			description:   "Success",
			payload:       []byte(`{"ts":"2019-02-13T21:19:02.614191735Z"}`),
			expectedTime:  goodTime,
			expectedFound: true,
		},
		{
			description: "Unmarshal Payload Error",
			payload:     []byte("test"),
		},
		{
			description: "Empty Payload String Error",
			payload:     []byte(``),
		},
		{
			description: "Non-String Timestamp Error",
			payload:     []byte(`{"ts":5}`),
		},
		{
			description: "Parse Timestamp Error",
			payload:     []byte(`{"ts":"2345"}`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			time, found := getBirthDate(tc.payload)
			assert.Equal(time, tc.expectedTime)
			assert.Equal(found, tc.expectedFound)
		})
	}
}
