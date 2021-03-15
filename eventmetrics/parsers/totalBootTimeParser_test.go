package parsers

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/eventmetrics/queue"
	"github.com/xmidt-org/glaukos/message"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestCheckLatestPreviousEvent(t *testing.T) {
	tests := []struct {
		description    string
		event          message.Event
		previousEvent  message.Event
		latestBootTime int64
		eventRegex     *regexp.Regexp
		expectedEvent  message.Event
		expectedErr    error
	}{
		{
			description: "New reboot event returned",
			event: message.Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "50"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
		},
		{
			description: "New different event returned",
			event: message.Event{
				Dest:     "event:device-status/some-event",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "50"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: message.Event{
				Dest:     "event:device-status/some-event",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
		},
		{
			description: "Same boot-time as previous, not reboot event",
			event: message.Event{
				Dest:     "event:device-status/some-event",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
		},
		{
			description: "Same boot-time, reboot event",
			event: message.Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
		},
		{
			description: "Same boot time, both reboot events",
			event: message.Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 30,
			},
			previousEvent: message.Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 20,
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: message.Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 20,
			},
		},
		{
			description: "Empty previous event",
			event: message.Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 30,
			},
			previousEvent:  message.Event{},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: message.Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 30,
			},
		},
		{
			description: "Error parsing boot-time",
			event: message.Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "not-a-number"},
			},
			previousEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "50"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "50"},
			},
			expectedErr: message.ErrBootTimeParse,
		},
		{
			description: "Error-Newer boot-time found",
			event: message.Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: message.Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "30"},
			},
			latestBootTime: 40,
			eventRegex:     rebootRegex,
			expectedEvent:  message.Event{},
			expectedErr:    errNewerBootTime,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			event, err := checkLatestPreviousEvent(tc.event, tc.previousEvent, tc.latestBootTime, tc.eventRegex)
			assert.Equal(tc.expectedEvent, event)

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

func TestIsEventValid(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	currTime := func() time.Time {
		return now
	}
	tests := []struct {
		description string
		event       message.Event
		regex       *regexp.Regexp
		expectedRes bool
		expectedErr error
	}{
		{
			description: "Valid Event",
			event: message.Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
				BirthDate: now.UnixNano(),
			},
			regex:       rebootRegex,
			expectedRes: true,
		},
		{
			description: "Wrong Event Type",
			event: message.Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/online",
			},
			regex:       rebootRegex,
			expectedRes: false,
			expectedErr: errEventNotFound,
		},
		{
			description: "No boot time",
			event: message.Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
			},
			regex:       rebootRegex,
			expectedRes: false,
			expectedErr: message.ErrPastDate,
		},
		{
			description: "Invalid boot time",
			event: message.Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Add(-200 * time.Hour).Unix()),
				},
			},
			regex:       rebootRegex,
			expectedRes: false,
			expectedErr: message.ErrPastDate,
		},
		{
			description: "Invalid birthdate",
			event: message.Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
				BirthDate: now.Add(-200 * time.Hour).UnixNano(),
			},
			regex:       rebootRegex,
			expectedRes: false,
			expectedErr: message.ErrPastDate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			res, err := isEventValid(tc.event, tc.regex, currTime)
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

type testReboot struct {
	description         string
	latestRebootPending int64 // should be unix timestamp in nanoseconds
	msg                 wrp.Message
	beginTime           time.Time
	events              []message.Event
	expectedErr         error
	expectedBadParse    float64
}

func TestCalculateRebootTimeError(t *testing.T) {
	now := time.Now()
	tests := []testReboot{
		{
			description: "Destination Regex Mismatch",
			msg:         wrp.Message{Destination: "event/random-event"},
		},
		{
			description: "Non fully-manageable event",
			msg:         wrp.Message{Destination: "event:device-status/mac:112233445566/random-event"},
		},
		{
			description: "Get WRP info error",
			msg: wrp.Message{
				Destination: "event:device-status/mac:112233445566/fully-manageable/1613039294",
				Metadata: map[string]string{
					bootTimeKey: "not-a-number",
				},
			},
			expectedErr: message.ErrBootTimeParse,
		},
		{
			description: "No previous events",
			expectedErr: errEventNotFound,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			events: []message.Event{},
		},
		{
			description: "No previous reboot-pending event",
			expectedErr: errEventNotFound,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			events: []message.Event{
				message.Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/online",
					TransactionUUID: "testOnline",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-1 * time.Minute).UnixNano(),
				},
			},
		},
		{
			description:      "Newer boot-time found",
			expectedErr:      errNewerBootTime,
			expectedBadParse: 1.0,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			events: []message.Event{
				message.Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/some-event",
					TransactionUUID: "testReboot",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(1 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-1 * time.Minute).UnixNano(),
				},
			},
		},
		{
			description: "Invalid Restart Time",
			expectedErr: errInvalidRestartTime,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			events: []message.Event{
				message.Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
					TransactionUUID: "testReboot",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
					},
					BirthDate: now.Add(1 * time.Minute).UnixNano(),
				},
			},
		},
		{
			description: "Missed reboot-pending event",
			expectedErr: errEventNotFound,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			events: []message.Event{
				message.Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
					TransactionUUID: "testReboot",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-2 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-1 * time.Minute).UnixNano(),
				},
				message.Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/online",
					TransactionUUID: "testOnline",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-1 * time.Minute).UnixNano(),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			client := new(mockEventClient)
			client.On("GetEvents", mock.Anything).Return(tc.events)
			p := xmetricstest.NewProvider(&xmetrics.Options{})
			m := Measures{
				UnparsableEventsCount: p.NewCounter("unparsable_events"),
			}
			b := RebootTimeParser{
				Measures: m,
				Client:   client,
				Logger:   log.NewNopLogger(),
				Label:    "reboot_to_manageable_duration",
			}

			var begin time.Time

			if tc.beginTime.IsZero() {
				begin = now
			} else {
				begin = tc.beginTime
			}
			time, err := b.calculateRestartTime(queue.WrpWithTime{Message: tc.msg, Beginning: begin})
			if tc.expectedErr != nil {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			}
			assert.Equal(-1.0, time)
			p.Assert(t, "unparsable_events", ParserLabel, "reboot_to_manageable_duration", ReasonLabel, eventBootTimeErr)(xmetricstest.Value(tc.expectedBadParse))
		})
	}
}

func TestCalculateRebootTimeSuccess(t *testing.T) {
	var (
		assert      = assert.New(t)
		eventClient = new(mockEventClient)
		p           = xmetricstest.NewProvider(&xmetrics.Options{})
		now         = time.Now()
		m           = Measures{
			UnparsableEventsCount: p.NewCounter("unparsable_events"),
		}
		b = RebootTimeParser{
			Measures: m,
			Client:   eventClient,
			Logger:   log.NewNopLogger(),
			Label:    "reboot_to_manageable_duration",
		}
	)

	test := testReboot{
		description: "Success",
		msg: wrp.Message{
			Type:            wrp.SimpleEventMessageType,
			Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
			TransactionUUID: "123abc",
			Metadata: map[string]string{
				bootTimeKey: fmt.Sprint(now.Unix()),
			},
		},
		latestRebootPending: now.Add(-1 * time.Minute).UnixNano(),
		events: []message.Event{
			message.Event{
				MsgType:         4,
				Dest:            "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
				TransactionUUID: "testReboot",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Add(-2 * time.Minute).Unix()),
				},
				BirthDate: now.Add(-6 * time.Minute).UnixNano(),
			},
			message.Event{
				MsgType:         4,
				Dest:            "event:device-status/mac:112233445566/online",
				TransactionUUID: "testOnline",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				BirthDate: now.Add(-2 * time.Minute).UnixNano(),
			},
			message.Event{
				MsgType:         4,
				Dest:            "event:device-status/mac:112233445566/reboot-pending/1122556/2s",
				TransactionUUID: "rebootEventFound",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				BirthDate: now.Add(-1 * time.Minute).UnixNano(),
			},
			message.Event{
				MsgType:         4,
				Dest:            "event:device-status/mac:112233445566/online",
				TransactionUUID: "testOnline",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
				},
				BirthDate: now.Add(-4 * time.Minute).UnixNano(),
			},
			message.Event{
				MsgType:         4,
				Dest:            "event:device-status/mac:112233445566/online",
				TransactionUUID: "emptyMetadata",
				Metadata:        map[string]string{},
				BirthDate:       now.Add(-5 * time.Minute).UnixNano(),
			},
		},
	}

	eventClient.On("GetEvents", mock.Anything).Return(test.events)

	res, err := b.calculateRestartTime(queue.WrpWithTime{Message: test.msg, Beginning: now})
	assert.Nil(err)
	assert.Equal(now.Sub(time.Unix(0, test.latestRebootPending)).Seconds(), res)
	p.Assert(t, "unparsable_events", ParserLabel, "reboot_to_manageable_duration", ReasonLabel, eventBootTimeErr)(xmetricstest.Value(0.0))

}
