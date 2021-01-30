package parsing

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestCheckOnlineEvent(t *testing.T) {
	dest := "event:device-status/mac:112233445566/online"
	assert := assert.New(t)
	now := time.Now()

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
					bootTimeKey: fmt.Sprintf("%d", now.Add(-3*time.Second).Unix()),
				},
				Dest:            dest,
				TransactionUUID: "random",
			},
			currentUUID:      "test",
			previousBootTime: now.Add(-1 * time.Minute).Unix(),
			latestBootTime:   now.Unix(),
			expectedBootTime: now.Add(-3 * time.Second).Unix(),
		},
		{
			description: "Old boot time",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Add(-1*time.Minute).Unix()),
				},
				Dest:            dest,
				TransactionUUID: "random",
			},
			currentUUID:      "test",
			previousBootTime: now.Add(-3 * time.Second).Unix(),
			latestBootTime:   now.Unix(),
			expectedBootTime: now.Add(-3 * time.Second).Unix(),
		},
		{
			description: "Error-Newer boot time found",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Unix()),
				},
				Dest:            dest,
				TransactionUUID: "random",
			},
			currentUUID:      "test",
			previousBootTime: now.Add(-1 * time.Minute).Unix(),
			latestBootTime:   now.Add(-3 * time.Second).Unix(),
			expectedBootTime: -1,
			expectedErr:      errors.New("found newer boot-time"),
		},
		{
			description: "Error-Same boot time found",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Unix()),
				},
				Dest:            dest,
				TransactionUUID: "random",
			},
			currentUUID:      "test",
			previousBootTime: now.Add(-3 * time.Second).Unix(),
			latestBootTime:   now.Unix(),
			expectedBootTime: -1,
			expectedErr:      errors.New("found same boot-time"),
		},
		{
			description: "Same boot time & same transactionUUID",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Add(-3*time.Second).Unix()),
				},
				Dest:            dest,
				TransactionUUID: "test",
			},
			currentUUID:      "test",
			previousBootTime: now.Add(-1 * time.Minute).Unix(),
			latestBootTime:   now.Add(-3 * time.Second).Unix(),
			expectedBootTime: now.Add(-1 * time.Minute).Unix(),
		},
		{
			description: "Current Event Boot Time & TransactionID",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Unix()),
				},
				Dest:            dest,
				TransactionUUID: "test",
			},
			currentUUID:      "test",
			previousBootTime: now.Add(-1 * time.Minute).Unix(),
			latestBootTime:   now.Unix(),
			expectedBootTime: now.Add(-1 * time.Minute).Unix(),
		},
		{
			description: "Not online event",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Add(-3*time.Second).Unix()),
				},
				Dest:            "event:device-status/mac:112233445566/random-event",
				TransactionUUID: "test",
			},
			currentUUID:      "test",
			previousBootTime: now.Add(-3 * time.Second).Unix(),
			latestBootTime:   now.Unix(),
			expectedBootTime: now.Add(-3 * time.Second).Unix(),
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
	now := time.Now()

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
					bootTimeKey: fmt.Sprintf("%d", now.Add(-3*time.Second).Unix()),
				},
				Dest:      dest,
				BirthDate: now.Add(-1 * time.Second).Unix(),
			},
			previousBootTime:  now.Add(-3 * time.Second).Unix(),
			latestBirthDate:   now.Unix(),
			expectedBirthDate: now.Unix(),
		},
		{
			description: "Same birthdate",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Add(-3*time.Second).Unix()),
				},
				Dest:      dest,
				BirthDate: now.Add(-1 * time.Second).Unix(),
			},
			previousBootTime:  now.Add(-3 * time.Second).Unix(),
			latestBirthDate:   now.Add(-1 * time.Second).Unix(),
			expectedBirthDate: now.Add(-1 * time.Second).Unix(),
		},
		{
			description: "Older birthdate",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Add(-3*time.Second).Unix()),
				},
				Dest:      dest,
				BirthDate: now.Add(-5 * time.Second).Unix(),
			},
			previousBootTime:  now.Add(-3 * time.Second).Unix(),
			latestBirthDate:   now.Add(-1 * time.Second).Unix(),
			expectedBirthDate: now.Add(-1 * time.Second).Unix(),
		},
		{
			description: "Wrong Boot time",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Add(-3*time.Second).Unix()),
				},
				Dest:      dest,
				BirthDate: now.Add(-5 * time.Second).Unix(),
			},
			previousBootTime:  now.Unix(),
			latestBirthDate:   now.Add(-10 * time.Second).Unix(),
			expectedBirthDate: now.Add(-10 * time.Second).Unix(),
		},
		{
			description: "Not Offline Event",
			event: Event{
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Unix()),
				},
				Dest:      "event:device-status/mac:112233445566/random-event",
				BirthDate: now.Add(-5 * time.Second).Unix(),
			},
			previousBootTime:  now.Unix(),
			latestBirthDate:   now.Add(-10 * time.Second).Unix(),
			expectedBirthDate: now.Add(-10 * time.Second).Unix(),
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

type test struct {
	description            string
	latestBootTime         time.Time
	latestOfflineBirthdate time.Time
	expectedTime           float64
	msg                    wrp.Message
	events                 []Event
	expectedErr            bool
	expectedBadParse       float64
}

func TestCalculateRestartTimeError(t *testing.T) {
	assert := assert.New(t)

	now := time.Now()
	tests := []test{
		{
			description:            "Destination Regex Mismatch",
			latestBootTime:         now,
			latestOfflineBirthdate: now.Add(-5 * time.Second),
			expectedTime:           -1,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Destination: "event/random-event",
			},
		},
		{
			description:    "Non-online event",
			latestBootTime: now,
			expectedTime:   -1,
			msg: wrp.Message{
				Type:        wrp.SimpleEventMessageType,
				Destination: "event:device-status/mac:112233445566/random-event",
			},
		},
		{
			description:    "No offline events",
			latestBootTime: now,
			expectedTime:   -1,
			expectedErr:    true,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/online",
				TransactionUUID: "123abc",
			},
			events: []Event{
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/online",
					TransactionUUID: "testOnline",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprintf("%d", now.Add(-1*time.Minute).Unix()),
					},
					BirthDate: now.Add(-5 * time.Second).Unix(),
				},
			},
		},
		{
			description:    "No previous online events",
			latestBootTime: now,
			expectedTime:   -1,
			expectedErr:    true,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/online",
				TransactionUUID: "123abc",
			},
			events: []Event{
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/offline",
					TransactionUUID: "abcdefghi",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprintf("%d", now.Add(-1*time.Minute).Unix()),
					},
					BirthDate: now.Add(-5 * time.Second).Unix(),
				},
			},
		},
		{
			description:    "No previous events",
			latestBootTime: now,
			expectedTime:   -1,
			expectedErr:    true,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/online",
				TransactionUUID: "123abc",
			},
			events: []Event{},
		},
		{
			description:    "Error with Event Boottime",
			latestBootTime: now.Add(-3 * time.Minute),
			expectedTime:   -1,
			expectedErr:    true,

			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/online",
				TransactionUUID: "123abc",
			},
			events: []Event{
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/online",
					TransactionUUID: "testOnline",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprintf("%d", now.Unix()),
					},
					BirthDate: now.Add(-5 * time.Second).Unix(),
				},
			},
			expectedBadParse: 1.0,
		},
		{
			description:            "Bad online event",
			latestBootTime:         now,
			latestOfflineBirthdate: now.Add(-1 * time.Second),
			expectedTime:           now.Sub(now.Add(-1 * time.Second)).Seconds(),
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/online",
				TransactionUUID: "123abc",
			},
			events: []Event{
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/online",
					TransactionUUID: "testOnline",
					Metadata:        map[string]string{},
				},
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/online",
					TransactionUUID: "testOnline2",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprintf("%d", now.Add(-2*time.Minute).Unix()),
					},
				},
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/offline",
					TransactionUUID: "testOffline2",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprintf("%d", now.Add(-2*time.Minute).Unix()),
					},
					BirthDate: now.Add(-1 * time.Second).Unix(),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client := new(mockEventClient)
			client.On("GetEvents", mock.Anything).Return(tc.events)
			p := xmetricstest.NewProvider(&xmetrics.Options{})
			b := BootTimeParser{
				UnparsableEventsCount: p.NewCounter("unparsable_events"),
				Client:                client,
				Logger:                log.NewNopLogger(),
			}

			tc.msg.Metadata = map[string]string{
				bootTimeKey: fmt.Sprintf("%d", tc.latestBootTime.Unix()),
			}

			time, err := b.calculateRestartTime(tc.msg)
			if tc.expectedErr {
				assert.NotNil(err)
			} else {
				assert.Nil(err)
			}

			assert.Equal(tc.expectedTime, time)
			p.Assert(t, "unparsable_events", ParserLabel, bootTimeParserLabel, ReasonLabel, eventBootTimeErr)(xmetricstest.Value(tc.expectedBadParse))
		})
	}
}

func TestCalculateRestartTimeSuccess(t *testing.T) {
	var (
		assert = assert.New(t)
		client = new(mockEventClient)
		p      = xmetricstest.NewProvider(&xmetrics.Options{})
		now    = time.Now()
		b      = BootTimeParser{
			UnparsableEventsCount: p.NewCounter("unparsable_events"),
			Client:                client,
			Logger:                log.NewNopLogger(),
		}
	)

	tc := test{
		description:            "Simple Success",
		latestBootTime:         now,
		latestOfflineBirthdate: now.Add(-5 * time.Second),
		expectedTime:           now.Sub(now.Add(-5 * time.Second)).Seconds(),
		msg: wrp.Message{
			Type:            wrp.SimpleEventMessageType,
			Destination:     "event:device-status/mac:112233445566/online",
			TransactionUUID: "123abc",
			Metadata: map[string]string{
				bootTimeKey: fmt.Sprintf("%d", now.Unix()),
			},
		},
		events: []Event{
			Event{
				MsgType:         4,
				Dest:            "event:device-status/mac:112233445566/online",
				TransactionUUID: "testOnline",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Add(-1*time.Minute).Unix()),
				},
				BirthDate: now.Add(-5 * time.Second).Unix(),
			},
			Event{
				MsgType:         4,
				Dest:            "event:device-status/mac:112233445566/offline",
				TransactionUUID: "testOffline",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Add(-1*time.Minute).Unix()),
				},
				BirthDate: now.Add(-5 * time.Second).Unix(),
			},
			Event{
				MsgType:         4,
				Dest:            "event:device-status/mac:112233445566/offline",
				TransactionUUID: "testOffline2",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprintf("%d", now.Add(-2*time.Minute).Unix()),
				},
				BirthDate: now.Add(-1 * time.Second).Unix(),
			},
		},
	}

	client.On("GetEvents", mock.Anything).Return(tc.events)

	tc.msg.Metadata = map[string]string{
		bootTimeKey: fmt.Sprintf("%d", tc.latestBootTime.Unix()),
	}

	time, err := b.calculateRestartTime(tc.msg)
	assert.Nil(err)
	assert.Equal(tc.expectedTime, time)
	p.Assert(t, "unparsable_events", ParserLabel, bootTimeParserLabel, ReasonLabel, eventBootTimeErr)(xmetricstest.Value(0.0))
}
