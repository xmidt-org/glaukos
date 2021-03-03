package parsing

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestCheckLatestPreviousEvent(t *testing.T) {
	tests := []struct {
		description    string
		event          Event
		previousEvent  Event
		latestBootTime int64
		eventRegex     *regexp.Regexp
		expectedEvent  Event
		expectedErr    error
	}{
		{
			description: "New reboot event returned",
			event: Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "50"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
		},
		{
			description: "New different event returned",
			event: Event{
				Dest:     "event:device-status/some-event",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "50"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: Event{
				Dest:     "event:device-status/some-event",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
		},
		{
			description: "Same boot-time as previous, not reboot event",
			event: Event{
				Dest:     "event:device-status/some-event",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
		},
		{
			description: "Same boot-time, reboot event",
			event: Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
		},
		{
			description: "Same boot time, both reboot events",
			event: Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 30,
			},
			previousEvent: Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 20,
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 30,
			},
		},
		{
			description: "Empty previous event",
			event: Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 30,
			},
			previousEvent:  Event{},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: Event{
				Dest:      "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata:  map[string]string{bootTimeKey: "60"},
				BirthDate: 30,
			},
		},
		{
			description: "Error parsing boot-time",
			event: Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "not-a-number"},
			},
			previousEvent: Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "50"},
			},
			latestBootTime: 70,
			eventRegex:     rebootRegex,
			expectedEvent: Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "50"},
			},
			expectedErr: errBootTimeParse,
		},
		{
			description: "Error-Newer boot-time found",
			event: Event{
				Dest:     "event:device-status/mac:112233445566/reboot-pending/1612424775/2s",
				Metadata: map[string]string{bootTimeKey: "60"},
			},
			previousEvent: Event{
				Dest:     "event:device-status/mac:112233445566/online",
				Metadata: map[string]string{bootTimeKey: "30"},
			},
			latestBootTime: 40,
			eventRegex:     rebootRegex,
			expectedEvent:  Event{},
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
				assert.Contains(err.Error(), tc.expectedErr.Error())
			}
		})
	}
}

func TestIsDateValid(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)

	currFunc := func() time.Time {
		return now
	}

	tests := []struct {
		description  string
		pastBuffer   time.Duration
		futureBuffer time.Duration
		testTime     time.Time
		expectedRes  bool
	}{
		{
			description:  "Valid Time",
			pastBuffer:   time.Hour,
			futureBuffer: 30 * time.Minute,
			testTime:     now.Add(2 * time.Minute),
			expectedRes:  true,
		},
		{
			description:  "Unix Time 0",
			pastBuffer:   time.Hour,
			futureBuffer: 30 * time.Minute,
			testTime:     time.Unix(0, 0),
			expectedRes:  false,
		},
		{
			description:  "Before unix Time 0",
			pastBuffer:   time.Hour,
			futureBuffer: 30 * time.Minute,
			testTime:     time.Unix(-10, 0),
			expectedRes:  false,
		},
		{
			description:  "Negative past buffer",
			pastBuffer:   -1 * time.Hour,
			futureBuffer: 30 * time.Minute,
			testTime:     now.Add(2 * time.Minute),
			expectedRes:  true,
		},
		{
			description:  "0 buffers",
			pastBuffer:   0,
			futureBuffer: 0,
			testTime:     now.Add(2 * time.Minute),
			expectedRes:  false,
		},
		{
			description:  "Equal time",
			pastBuffer:   0,
			futureBuffer: 0,
			testTime:     now,
			expectedRes:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			valid := isDateValid(currFunc, tc.pastBuffer, tc.futureBuffer, tc.testTime)
			assert.Equal(tc.expectedRes, valid)
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
		event       Event
		regex       *regexp.Regexp
		expectedRes bool
		expectedErr error
	}{
		{
			description: "Valid Event",
			event: Event{
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
			event: Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/online",
			},
			regex:       rebootRegex,
			expectedRes: false,
			expectedErr: errRestartTime,
		},
		{
			description: "No boot time",
			event: Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
			},
			regex:       rebootRegex,
			expectedRes: false,
			expectedErr: errRestartTime,
		},
		{
			description: "Invalid boot time",
			event: Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Add(-200 * time.Hour).Unix()),
				},
			},
			regex:       rebootRegex,
			expectedRes: false,
			expectedErr: errRestartTime,
		},
		{
			description: "Invalid birthdate",
			event: Event{
				MsgType: 4,
				Dest:    "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
				BirthDate: now.Add(-200 * time.Hour).UnixNano(),
			},
			regex:       rebootRegex,
			expectedRes: false,
			expectedErr: errRestartTime,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			res, err := isEventValid(tc.event, tc.regex, currTime)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
			} else {
				assert.Contains(err.Error(), tc.expectedErr.Error())
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
	events              []Event
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
			expectedErr: errBootTimeParse,
		},
		{
			description: "No previous events",
			expectedErr: errRestartTime,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			events: []Event{},
		},
		{
			description: "No previous reboot-pending event",
			expectedErr: errRestartTime,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			events: []Event{
				Event{
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
			description: "Reboot-pending boot-time greater than latest boot-time ",
			expectedErr: errNewerBootTime,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			events: []Event{
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
					TransactionUUID: "testReboot",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(1 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-1 * time.Minute).UnixNano(),
				},
			},
		},
		{
			description: "Missed reboot-pending event",
			expectedErr: errRestartTime,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			events: []Event{
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
					TransactionUUID: "testReboot",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-2 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-1 * time.Minute).UnixNano(),
				},
				Event{
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
			b := TotalBootTimeParser{
				Measures: m,
				Client:   client,
				Logger:   log.NewNopLogger(),
			}

			var begin time.Time

			if tc.beginTime.IsZero() {
				begin = now
			} else {
				begin = tc.beginTime
			}
			time, err := b.calculateRestartTime(queue.WrpWithTime{Message: tc.msg, Beginning: begin})
			if tc.expectedErr != nil {
				assert.Contains(err.Error(), tc.expectedErr.Error())
			}
			assert.Equal(-1.0, time)
			p.Assert(t, "unparsable_events", ParserLabel, bootTimeParserLabel, ReasonLabel, eventBootTimeErr)(xmetricstest.Value(tc.expectedBadParse))
		})
	}
}

func TestCalculateRebootTimeSuccess(t *testing.T) {
	var (
		assert = assert.New(t)
		client = new(mockEventClient)
		p      = xmetricstest.NewProvider(&xmetrics.Options{})
		now    = time.Now()
		m      = Measures{
			UnparsableEventsCount: p.NewCounter("unparsable_events"),
		}
		b = TotalBootTimeParser{
			Measures: m,
			Client:   client,
			Logger:   log.NewNopLogger(),
		}
	)

	tests := []testReboot{
		{
			description: "Success",
			expectedErr: errRestartTime,
			msg: wrp.Message{
				Type:            wrp.SimpleEventMessageType,
				Destination:     "event:device-status/mac:112233445566/fully-manageable/1613039294",
				TransactionUUID: "123abc",
				Metadata: map[string]string{
					bootTimeKey: fmt.Sprint(now.Unix()),
				},
			},
			latestRebootPending: now.Add(-1 * time.Minute).UnixNano(),
			events: []Event{
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/reboot-pending/1613033276/2s",
					TransactionUUID: "testReboot",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-2 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-6 * time.Minute).UnixNano(),
				},
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/online",
					TransactionUUID: "testOnline",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-2 * time.Minute).UnixNano(),
				},
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/reboot-pending/1122556/2s",
					TransactionUUID: "rebootEventFound",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-1 * time.Minute).UnixNano(),
				},
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/online",
					TransactionUUID: "testOnline",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
					},
					BirthDate: now.Add(-4 * time.Minute).UnixNano(),
				},
				Event{
					MsgType:         4,
					Dest:            "event:device-status/mac:112233445566/online",
					TransactionUUID: "emptyMetadata",
					Metadata:        map[string]string{},
					BirthDate:       now.Add(-5 * time.Minute).UnixNano(),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			client.On("GetEvents", mock.Anything).Return(tc.events)

			res, err := b.calculateRestartTime(queue.WrpWithTime{Message: tc.msg, Beginning: now})
			assert.Nil(err)
			assert.Equal(now.Sub(time.Unix(0, tc.latestRebootPending)).Seconds(), res)
			p.Assert(t, "unparsable_events", ParserLabel, bootTimeParserLabel, ReasonLabel, eventBootTimeErr)(xmetricstest.Value(0.0))
		})
	}
}
