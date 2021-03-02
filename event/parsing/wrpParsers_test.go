package parsing

import (
	"errors"
	"testing"
	"time"

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
			description:      "No Metadata",
			msg:              Event{},
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
			expectedErr: errParseDeviceID,
		},
		{
			description: "Invalid ID-missing event type",
			destination: "event:device-status/mac:123",
			expectedErr: errParseDeviceID,
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
			if err != nil || tc.expectedErr != nil {
				assert.True(errors.Is(err, tc.expectedErr))
			}

		})
	}
}

func TestGetValidBirthDate(t *testing.T) {
	payload := []byte(`{"ts":"2019-02-13T21:19:02.614191735Z"}`)
	testassert := assert.New(t)
	goodTime, err := time.Parse(time.RFC3339Nano, "2019-02-13T21:19:02.614191735Z")
	testassert.Nil(err)
	currTime, err := time.Parse(time.RFC3339Nano, "2019-02-13T21:21:21.614191735Z")
	testassert.Nil(err)

	tests := []struct {
		description       string
		fakeNow           time.Time
		payload           []byte
		expectedBirthDate time.Time
		expectedErr       error
	}{
		{
			description:       "Success",
			fakeNow:           currTime,
			payload:           payload,
			expectedBirthDate: goodTime,
		},
		{
			description:       "Success No Birthdate in Payload",
			fakeNow:           currTime,
			payload:           nil,
			expectedBirthDate: currTime,
		},
		{
			description: "Future Birthdate Error",
			fakeNow:     currTime.Add(-5 * time.Hour),
			payload:     payload,
			expectedErr: errFutureBirthDate,
		},
		{
			description: "Past Birthdate Error",
			fakeNow:     currTime.Add(200 * time.Hour),
			payload:     payload,
			expectedErr: errPastBirthDate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			currTime := func() time.Time {
				return tc.fakeNow
			}
			b, err := GetValidBirthDate(currTime, tc.payload)
			assert.Equal(tc.expectedBirthDate, b)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.Contains(err.Error(), tc.expectedErr.Error())
			}
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
