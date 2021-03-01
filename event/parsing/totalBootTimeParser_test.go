package parsing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestGetPreviousBootTime(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		description      string
		event            Event
		previousBootTime int64
		latestBootTime   int64
		msg              wrp.Message
		expectedResult   int64
		expectedErr      bool
		expectedNew      bool
	}{
		{
			description: "New boot-time found",
			event: Event{
				Metadata:        map[string]string{bootTimeKey: "52"},
				TransactionUUID: "abc",
			},
			previousBootTime: 50,
			latestBootTime:   56,
			msg: wrp.Message{
				TransactionUUID: "123",
			},
			expectedResult: 52,
			expectedNew:    true,
		},
		{
			description: "No new boot-time",
			event: Event{
				Metadata: map[string]string{
					"/boot-time": "20",
				},
				TransactionUUID: "abc",
			},
			previousBootTime: 30,
			latestBootTime:   56,
			msg: wrp.Message{
				TransactionUUID: "123",
			},
			expectedResult: 30,
			expectedNew:    false,
		},
		{
			description: "Error getting boottime",
			event: Event{
				Metadata:        map[string]string{bootTimeKey: "not-a-number"},
				TransactionUUID: "abc",
			},
			previousBootTime: 52,
			latestBootTime:   56,
			msg: wrp.Message{
				TransactionUUID: "123",
			},
			expectedResult: 52,
			expectedNew:    false,
			expectedErr:    true,
		},
		{
			description: "Boot-time key not present",
			event: Event{
				Metadata:        map[string]string{},
				TransactionUUID: "abc",
			},
			previousBootTime: 52,
			latestBootTime:   56,
			msg: wrp.Message{
				TransactionUUID: "123",
			},
			expectedResult: 52,
			expectedNew:    false,
		},
		{
			description: "Error-boot-time more recent than current",
			event: Event{
				Metadata:        map[string]string{bootTimeKey: "70"},
				TransactionUUID: "abc",
			},
			previousBootTime: 52,
			latestBootTime:   56,
			msg: wrp.Message{
				TransactionUUID: "123",
			},
			expectedResult: -1,
			expectedNew:    false,
			expectedErr:    true,
		},
		{
			description: "Same UUID more recent bootime",
			event: Event{
				Metadata:        map[string]string{bootTimeKey: "60"},
				TransactionUUID: "123",
			},
			previousBootTime: 52,
			latestBootTime:   70,
			msg: wrp.Message{
				TransactionUUID: "123",
			},
			expectedResult: 52,
			expectedNew:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			bootTimeFound, newlyFound, err := getPreviousBootTime(tc.event, tc.previousBootTime, tc.latestBootTime, tc.msg)
			assert.Equal(tc.expectedResult, bootTimeFound)
			assert.Equal(tc.expectedNew, newlyFound)
			if tc.expectedErr {
				assert.NotNil(err)
			} else {
				assert.Nil(err)
			}
		})
	}

}

func TestGetLastRebootEvent(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		description           string
		event                 Event
		latestRebootBirthDate int64
		expectedResult        int64
	}{
		{
			description: "Newer birthdate found",
			event: Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				BirthDate: 50,
			},
			latestRebootBirthDate: 20,
			expectedResult:        50,
		},
		{
			description: "No new birthdate found",
			event: Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				BirthDate: 30,
			},
			latestRebootBirthDate: 60,
			expectedResult:        60,
		},
		{
			description: "Not a reboot-pending event",
			event: Event{
				Dest:      "event:device-status/mac:112233445566/some-event/1122221111",
				BirthDate: 50,
			},
			latestRebootBirthDate: 30,
			expectedResult:        30,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			res, err := getLastRebootEvent(tc.event, tc.latestRebootBirthDate)
			assert.Equal(tc.expectedResult, res)
			assert.Nil(err)
		})
	}
}

func TestCalculateRestartTime(t *testing.T) {

}
