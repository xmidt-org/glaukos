package parsing

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckOnlineEvent(t *testing.T) {
	dest := "event:device-status/mac:112233445566/online"
	assert := assert.New(t)

	tests := []struct {
		description      string
		event            Event
		currentUUID      string
		previousBootTime int64
		latestBootTime   int64
		expectedBootTime int64
		expectedErr      error
	}{
		{
			description: "More recent boot time",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1609459300",
				},
				Dest:            dest,
				TransactionUUID: "random",
			},
			currentUUID:      "test",
			previousBootTime: 1609459200,
			latestBootTime:   1611783626,
			expectedBootTime: 1609459300,
		},
		{
			description: "Old boot time",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1608238911",
				},
				Dest:            dest,
				TransactionUUID: "random",
			},
			currentUUID:      "test",
			previousBootTime: 1609459200,
			latestBootTime:   1611783626,
			expectedBootTime: 1609459200,
		},
		{
			description: "Error-Newer boot time found",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1622783626",
				},
				Dest:            dest,
				TransactionUUID: "random",
			},
			currentUUID:      "test",
			previousBootTime: 1609459200,
			latestBootTime:   1611783626,
			expectedBootTime: -1,
			expectedErr:      errors.New("found newer boot-time"),
		},
		{
			description: "Error-Same boot time found",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1611783626",
				},
				Dest:            dest,
				TransactionUUID: "random",
			},
			currentUUID:      "test",
			previousBootTime: 1609459200,
			latestBootTime:   1611783626,
			expectedBootTime: -1,
			expectedErr:      errors.New("found same boot-time"),
		},
		{
			description: "Same boot time & same transactionUUID",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1611783626",
				},
				Dest:            dest,
				TransactionUUID: "test",
			},
			currentUUID:      "test",
			previousBootTime: 1609459200,
			latestBootTime:   1611783626,
			expectedBootTime: 1611783626,
		},
		{
			description: "Not online event",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1611783626",
				},
				Dest:            "event:device-status/mac:112233445566/random-event",
				TransactionUUID: "test",
			},
			currentUUID:      "test",
			previousBootTime: 1609459200,
			latestBootTime:   1611783626,
			expectedBootTime: 1609459200,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			bootTime, err := checkOnlineEvent(tc.event, tc.currentUUID, tc.previousBootTime, tc.latestBootTime)
			assert.Equal(tc.expectedBootTime, bootTime)
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestCheckOfflineEvent(t *testing.T) {
	dest := "event:device-status/mac:112233445566/offline"
	assert := assert.New(t)

	tests := []struct {
		description       string
		event             Event
		previousBootTime  int64
		latestBirthDate   int64
		expectedBirthDate int64
	}{
		{
			description: "More recent birthdate",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1609459200",
				},
				Dest:      dest,
				BirthDate: 1609459600,
			},
			previousBootTime:  1609459200,
			latestBirthDate:   1609459300,
			expectedBirthDate: 1609459600,
		},
		{
			description: "Same birthdate",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1609459200",
				},
				Dest:      dest,
				BirthDate: 1609459300,
			},
			previousBootTime:  1609459200,
			latestBirthDate:   1609459300,
			expectedBirthDate: 1609459300,
		},
		{
			description: "Older birthdate",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1609459200",
				},
				Dest:      dest,
				BirthDate: 1609459100,
			},
			previousBootTime:  1609459200,
			latestBirthDate:   1609459300,
			expectedBirthDate: 1609459300,
		},
		{
			description: "Wrong Boot time",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1609459300",
				},
				Dest:      dest,
				BirthDate: 1609459500,
			},
			previousBootTime:  1609459200,
			latestBirthDate:   1609459300,
			expectedBirthDate: 1609459300,
		},
		{
			description: "Not Offline Event",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: "1609459200",
				},
				Dest:      "event:device-status/mac:112233445566/random-event",
				BirthDate: 1609459800,
			},
			previousBootTime:  1609459200,
			latestBirthDate:   1609459300,
			expectedBirthDate: 1609459300,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			birthDate, err := checkOfflineEvent(tc.event, tc.previousBootTime, tc.latestBirthDate)
			assert.Equal(tc.expectedBirthDate, birthDate)
			assert.Nil(err)
		})
	}
}
