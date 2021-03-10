package metricparsers

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
	"github.com/xmidt-org/wrp-go/v3"
)

const bootTimeKey = "/boot-time"

func TestCreateNewTimeElapsedParser(t *testing.T) {
	logger := log.NewNopLogger()
	tests := []struct {
		description string
		config      TimeElapsedConfig
		name        string
		expectedErr error
	}{
		{
			description: "Success",
			config: TimeElapsedConfig{
				Name: "test1",
				InitialEvent: parsing.EventRule{
					Regex:     `.*/some-event$`,
					ValidFrom: -1 * time.Hour,
				},
				IncomingEvent: parsing.EventRule{
					Regex:     `.*/some-event2$`,
					ValidFrom: -1 * time.Hour,
				},
			},
			name: "random",
		},
		{
			description: "Error with initial validator",
			config: TimeElapsedConfig{
				InitialEvent: parsing.EventRule{
					Regex: `'(?=.*\d)'`,
				},
				IncomingEvent: parsing.EventRule{
					Regex: `.*/online$`,
				},
			},
			expectedErr: parsing.ErrInvalidRegex,
		},
		{
			description: "Error with end validator",
			config: TimeElapsedConfig{
				InitialEvent: parsing.EventRule{
					Regex: `.*/online$`,
				},
				IncomingEvent: parsing.EventRule{
					Regex: `'(?=.*\d)'`,
				},
			},
			expectedErr: parsing.ErrInvalidRegex,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			measures := Measures{}
			client := new(client.MockEventClient)
			parser, err := CreateNewTimeElapsedParser(tc.config, tc.name, client, logger, measures)
			testLogger := GetParserLogger(logger, tc.name)
			if tc.expectedErr == nil || err == nil {
				assert.Equal(tc.expectedErr, err)
				assert.Equal(tc.name, parser.label)
				assert.Equal(testLogger, parser.logger)
				assert.Equal(client, parser.client)
				assert.Equal(measures, parser.measures)
				assert.NotNil(parser.initialValidator)
				assert.NotNil(parser.endValidator)
			} else {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
				assert.Nil(parser)
			}
		})
	}
}

func TestParseNoHardwareFirmwareError(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTimeFunc := func() time.Time { return now }
	initialRule := parsing.EventRule{
		Regex:            ".*/event-1/",
		DuplicateAllowed: true,
		ValidFrom:        -2 * time.Hour,
	}
	endRule := parsing.EventRule{
		Regex:            ".*/event-2/",
		DuplicateAllowed: true,
		ValidFrom:        -2 * time.Hour,
	}

	currBirthdate := now.Add(-1 * time.Minute)
	codexBirthdate := now.Add(-2 * time.Minute)

	msg := queue.WrpWithTime{
		Message: wrp.Message{
			Destination: "event:device-status/mac:112233445566/event-2/1613039294",
			Metadata: map[string]string{
				bootTimeKey: fmt.Sprint(now.Unix()),
			},
		},
		Beginning: currBirthdate,
	}

	codexEvents := []client.Event{
		client.Event{
			Dest: "event:device-status/mac:112233445566/event-1/1613039294",
			Metadata: map[string]string{
				bootTimeKey: fmt.Sprint(now.Add(-4 * time.Minute).Unix()),
			},
			BirthDate: codexBirthdate.UnixNano(),
		},
	}

	assert := assert.New(t)
	client := new(client.MockEventClient)
	client.On("GetEvents", mock.Anything).Return(codexEvents)
	p := xmetricstest.NewProvider(&xmetrics.Options{})
	m := Measures{
		UnparsableEventsCount: p.NewCounter("unparsable_events"),
	}

	initialVal, err := parsing.NewEventValidation(initialRule, time.Hour, currTimeFunc)
	assert.Nil(err)
	endVal, err := parsing.NewEventValidation(endRule, time.Hour, currTimeFunc)
	assert.Nil(err)

	tep := TimeElapsedParser{
		initialValidator: initialVal,
		endValidator:     endVal,
		measures:         m,
		logger:           log.NewNopLogger(),
		label:            "test_TEP",
		client:           client,
	}

	err = tep.Parse(msg)
	assert.Nil(err)
	p.Assert(t, "unparsable_events", ParserLabel, "test_TEP", ReasonLabel, errNoFirmwareOrHardware)(xmetricstest.Value(1.0))
}

func TestTimeElapsedSuccess(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTimeFunc := func() time.Time { return now }
	initialRule := parsing.EventRule{
		Regex:            ".*/event-1/",
		DuplicateAllowed: true,
		ValidFrom:        -2 * time.Hour,
	}
	endRule := parsing.EventRule{
		Regex:            ".*/event-2/",
		DuplicateAllowed: true,
		ValidFrom:        -2 * time.Hour,
	}

	currBirthdate := now.Add(-1 * time.Minute)
	currBootTime := now
	codexBirthdate := now.Add(-2 * time.Minute)
	codexBootTime := now.Add(-4 * time.Minute)

	msg := queue.WrpWithTime{
		Message: wrp.Message{
			Destination: "event:device-status/mac:112233445566/event-2/1613039294",
			Metadata: map[string]string{
				bootTimeKey: fmt.Sprint(currBootTime.Unix()),
			},
		},
		Beginning: currBirthdate,
	}

	codexEvents := []client.Event{
		client.Event{
			Dest: "event:device-status/mac:112233445566/event-1/1613039294",
			Metadata: map[string]string{
				bootTimeKey: fmt.Sprint(codexBootTime.Unix()),
			},
			BirthDate: codexBirthdate.UnixNano(),
		},
		client.Event{
			Dest: "event:device-status/mac:112233445566/event-1/1613039294",
			Metadata: map[string]string{
				bootTimeKey: fmt.Sprint(now.Add(-5 * time.Minute).Unix()),
			},
			BirthDate: now.Add(-6 * time.Minute).UnixNano(),
		},
	}

	tests := []struct {
		description           string
		initialCalculateUsing parsing.TimeLocation
		endCalculateUsing     parsing.TimeLocation
		expectedRes           float64
	}{
		{
			description:           "Birthdate - birthdate",
			initialCalculateUsing: parsing.Birthdate,
			endCalculateUsing:     parsing.Birthdate,
			expectedRes:           currBirthdate.Sub(codexBirthdate).Seconds(),
		},
		{
			description:           "Birthdate - boottime",
			initialCalculateUsing: parsing.Boottime,
			endCalculateUsing:     parsing.Birthdate,
			expectedRes:           currBirthdate.Sub(codexBootTime).Seconds(),
		},
		{
			description:           "Boottime - birthdate",
			initialCalculateUsing: parsing.Birthdate,
			endCalculateUsing:     parsing.Boottime,
			expectedRes:           currBootTime.Sub(codexBirthdate).Seconds(),
		},
		{
			description:           "Boottime - boottime",
			initialCalculateUsing: parsing.Boottime,
			endCalculateUsing:     parsing.Boottime,
			expectedRes:           currBootTime.Sub(codexBootTime).Seconds(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			client := new(client.MockEventClient)
			client.On("GetEvents", mock.Anything).Return(codexEvents)
			p := xmetricstest.NewProvider(&xmetrics.Options{})
			m := Measures{
				UnparsableEventsCount: p.NewCounter("unparsable_events"),
			}

			if tc.initialCalculateUsing == parsing.Boottime {
				initialRule.CalculateUsing = "Boot-time"
			} else {
				initialRule.CalculateUsing = "Birthdate"
			}

			if tc.endCalculateUsing == parsing.Boottime {
				endRule.CalculateUsing = "Boot-time"
			} else {
				endRule.CalculateUsing = "Birthdate"
			}

			initialVal, err := parsing.NewEventValidation(initialRule, time.Hour, currTimeFunc)
			assert.Nil(err)
			endVal, err := parsing.NewEventValidation(endRule, time.Hour, currTimeFunc)
			assert.Nil(err)

			tep := TimeElapsedParser{
				initialValidator: initialVal,
				endValidator:     endVal,
				measures:         m,
				logger:           log.NewNopLogger(),
				label:            "test_TEP",
				client:           client,
			}

			time, err := tep.calculateTimeElapsed(msg)
			assert.Nil(err)
			assert.Equal(tc.expectedRes, time)
			p.Assert(t, "unparsable_events", ParserLabel, "test_TEP", ReasonLabel, errEventBootTime)(xmetricstest.Value(0.0))

		})
	}
}

func TestTimeElapsedCodexEventErrors(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTimeFunc := func() time.Time { return now }
	initialRule := parsing.EventRule{
		Regex:            ".*/event-1/",
		CalculateUsing:   "Boot-time",
		DuplicateAllowed: true,
		ValidFrom:        -2 * time.Hour,
	}

	endRule := parsing.EventRule{
		Regex:            ".*/event-2/",
		CalculateUsing:   "Boot-time",
		DuplicateAllowed: true,
		ValidFrom:        -2 * time.Hour,
	}

	tests := []struct {
		description      string
		msg              queue.WrpWithTime
		codexEvents      []client.Event
		initialRule      parsing.EventRule
		endRule          parsing.EventRule
		expectedErr      error
		expectedBadParse float64
	}{
		{
			description: "No previous events",
			msg: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/event-2/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Unix()),
					},
				},
			},
			initialRule: initialRule,
			endRule:     endRule,
			codexEvents: []client.Event{},
			expectedErr: errInvalidPrevEvent,
		},
		{
			description: "No initial events",
			msg: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/event-2/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Unix()),
					},
				},
			},
			initialRule: initialRule,
			endRule:     endRule,
			codexEvents: []client.Event{
				client.Event{
					Dest: "event:device-status/mac:112233445566/event-2/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-30 * time.Minute).Unix()),
					},
				},
			},
			expectedErr: errInvalidPrevEvent,
		},
		{
			description: "Boot-time newer than latest",
			msg: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/event-2/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Unix()),
					},
				},
			},
			initialRule: initialRule,
			endRule:     endRule,
			codexEvents: []client.Event{
				client.Event{
					Dest: "event:device-status/mac:112233445566/event-2/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(2 * time.Minute).Unix()),
					},
				},
			},
			expectedErr:      errNewerBootTime,
			expectedBadParse: 1.0,
		},
		{
			description: "Duplicate event not allowed",
			msg: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/event-2/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Unix()),
					},
				},
			},
			initialRule: parsing.EventRule{
				Regex:            ".*/event-1/",
				CalculateUsing:   "Boot-time",
				DuplicateAllowed: false,
				ValidFrom:        -2 * time.Hour,
			},
			endRule: endRule,
			codexEvents: []client.Event{
				client.Event{
					Dest: "event:device-status/mac:112233445566/event-1/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
					},
				},
				client.Event{
					Dest: "event:device-status/mac:112233445566/event-1/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
					},
				},
			},
			expectedErr:      errSameBootTime,
			expectedBadParse: 1.0,
		},
		{
			description: "Missed codex event",
			msg: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/event-2/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Unix()),
					},
				},
			},
			initialRule: parsing.EventRule{
				Regex:            ".*/event-1/",
				CalculateUsing:   "Boot-time",
				DuplicateAllowed: false,
				ValidFrom:        -2 * time.Hour,
			},
			endRule: endRule,
			codexEvents: []client.Event{
				client.Event{
					Dest: "event:device-status/mac:112233445566/random-event",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-1 * time.Minute).Unix()),
					},
				},
				client.Event{
					Dest: "event:device-status/mac:112233445566/event-1/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-2 * time.Minute).Unix()),
					},
				},
			},
			expectedErr: errInvalidPrevEvent,
		},
		{
			description: "Invalid Restart Time",
			msg: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/event-2/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Unix()),
					},
				},
			},
			initialRule: parsing.EventRule{
				Regex:            ".*/event-1/",
				CalculateUsing:   "Birthdate",
				DuplicateAllowed: false,
				ValidFrom:        -2 * time.Hour,
			},
			endRule: endRule,
			codexEvents: []client.Event{
				client.Event{
					Dest: "event:device-status/mac:112233445566/event-1/1613039294",
					Metadata: map[string]string{
						bootTimeKey: fmt.Sprint(now.Add(-2 * time.Minute).Unix()),
					},
					BirthDate: now.Add(time.Minute).UnixNano(),
				},
			},
			expectedErr: errInvalidTimeElapsed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			client := new(client.MockEventClient)
			client.On("GetEvents", mock.Anything).Return(tc.codexEvents)
			p := xmetricstest.NewProvider(&xmetrics.Options{})
			m := Measures{
				UnparsableEventsCount: p.NewCounter("unparsable_events"),
			}

			initialVal, err := parsing.NewEventValidation(tc.initialRule, time.Hour, currTimeFunc)
			assert.Nil(err)
			endVal, err := parsing.NewEventValidation(tc.endRule, time.Hour, currTimeFunc)
			assert.Nil(err)

			tep := TimeElapsedParser{
				initialValidator: initialVal,
				endValidator:     endVal,
				measures:         m,
				logger:           log.NewNopLogger(),
				label:            "test_TEP",
				client:           client,
			}

			time, err := tep.calculateTimeElapsed(tc.msg)
			if tc.expectedErr != nil {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			} else {
				assert.Equal(tc.expectedErr, err)
			}
			assert.Equal(-1.0, time)
			p.Assert(t, "unparsable_events", ParserLabel, "test_TEP", ReasonLabel, errEventBootTime)(xmetricstest.Value(tc.expectedBadParse))
		})
	}
}

func TestTimeElapsedParsingErrors(t *testing.T) {
	logger := log.NewNopLogger()
	tests := []struct {
		description string
		wrp         queue.WrpWithTime
		wrpIsValid  bool
		expectedErr error
	}{
		{
			description: "Invalid Event Dest",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "mac:112233445566/fully-manageable/1615223357",
				},
			},
			wrpIsValid:  true,
			expectedErr: errInvalidEventDest,
		},
		{
			description: "Invalid WRP",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/fully-manageable/1615223357",
				},
			},
			wrpIsValid: false,
		},
		{
			description: "Parse WRP error",
			wrp: queue.WrpWithTime{
				Message: wrp.Message{
					Destination: "event:device-status/mac:112233445566/fully-manageable/1615223357",
				},
			},
			expectedErr: errInvalidBootTime,
			wrpIsValid:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			endVal := new(parsing.MockEventValidation)
			endVal.On("IsWRPValid", tc.wrp).Return(tc.wrpIsValid, nil)
			tep := TimeElapsedParser{endValidator: endVal, logger: logger}
			res, err := tep.calculateTimeElapsed(tc.wrp)
			assert.Equal(-1.0, res)
			if tc.expectedErr != nil {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			} else {
				assert.Equal(tc.expectedErr, err)
			}
		})
	}
}

func TestCheckLatestInitialEvent(t *testing.T) {
	t.Run("Different boot-times", testInitialDiffBootTime)
	t.Run("Same boot-times", testInitialSameBootTime)
}

func testInitialDiffBootTime(t *testing.T) {
	tests := []struct {
		description      string
		event            client.Event
		previousEvent    client.Event
		latestBootTime   int64
		duplicateAllowed bool
		expectedEvent    client.Event
		expectedErr      error
	}{
		{
			description: "Error parsing boot-time",
			event: client.Event{
				Metadata: map[string]string{},
				Dest:     "some-event",
			},
			previousEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "50"},
				Dest:     "some-event",
			},
			latestBootTime: 70,
			expectedEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "50"},
				Dest:     "some-event",
			},
			expectedErr: errInvalidBootTime,
		},
		{
			description: "Invalid boot-time",
			event: client.Event{
				Metadata: map[string]string{bootTimeKey: "-1"},
				Dest:     "some-event",
			},
			previousEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "50"},
				Dest:     "some-event",
			},
			latestBootTime: 70,
			expectedEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "50"},
				Dest:     "some-event",
			},
			expectedErr: errInvalidBootTime,
		},
		{
			description: "Boot-time > latest",
			event: client.Event{
				Metadata: map[string]string{bootTimeKey: "100"},
				Dest:     "some-event",
			},
			previousEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "50"},
				Dest:     "some-event",
			},
			latestBootTime: 70,
			expectedEvent:  client.Event{},
			expectedErr:    errNewerBootTime,
		},
		{
			description: "New event returned",
			event: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
			previousEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "50"},
				Dest:     "some-event",
			},
			latestBootTime: 70,
			expectedEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
		},
		{
			description: "Previous event returned",
			event: client.Event{
				Metadata: map[string]string{bootTimeKey: "50"},
				Dest:     "some-event",
			},
			previousEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
			latestBootTime: 70,
			expectedEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			tep := TimeElapsedParser{}
			res, err := tep.checkLatestInitialEvent(tc.event, tc.previousEvent, tc.latestBootTime)
			assert.Equal(tc.expectedEvent, res)
			if tc.expectedErr != nil {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			} else {
				assert.Equal(tc.expectedErr, err)
			}
		})
	}
}

func testInitialSameBootTime(t *testing.T) {
	tests := []struct {
		description      string
		event            client.Event
		previousEvent    client.Event
		latestBootTime   int64
		currEventIsValid bool
		prevEventIsValid bool
		duplicateAllowed bool
		expectedEvent    client.Event
		expectedErr      error
	}{
		{
			description: "Duplicates not allowed",
			event: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
			previousEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
			latestBootTime:   70,
			expectedEvent:    client.Event{},
			expectedErr:      errSameBootTime,
			currEventIsValid: true,
			prevEventIsValid: true,
			duplicateAllowed: false,
		},
		{
			description: "Duplicates not allowed, different dest",
			event: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
			previousEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "other-event",
			},
			latestBootTime: 70,
			expectedEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
			currEventIsValid: true,
			prevEventIsValid: false,
			duplicateAllowed: false,
		},
		{
			description: "Previous event not desired type",
			event: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
			previousEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "other-event",
			},
			latestBootTime: 70,
			expectedEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
			currEventIsValid: true,
			prevEventIsValid: false,
			duplicateAllowed: true,
		},
		{
			description: "Curr event is older",
			event: client.Event{
				Metadata:  map[string]string{bootTimeKey: "60"},
				Dest:      "some-event",
				BirthDate: 30,
			},
			previousEvent: client.Event{
				Metadata:  map[string]string{bootTimeKey: "60"},
				Dest:      "some-event",
				BirthDate: 40,
			},
			latestBootTime: 70,
			expectedEvent: client.Event{
				Metadata:  map[string]string{bootTimeKey: "60"},
				Dest:      "some-event",
				BirthDate: 30,
			},
			currEventIsValid: true,
			prevEventIsValid: true,
			duplicateAllowed: true,
		},
		{
			description: "Curr event type is wrong",
			event: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "some-event",
			},
			previousEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "other-event",
			},
			latestBootTime: 70,
			expectedEvent: client.Event{
				Metadata: map[string]string{bootTimeKey: "60"},
				Dest:     "other-event",
			},
			currEventIsValid: false,
			prevEventIsValid: true,
			duplicateAllowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			val := new(parsing.MockEventValidation)
			val.On("DuplicateAllowed").Return(tc.duplicateAllowed)
			val.On("ValidateType", tc.event.Dest).Return(tc.currEventIsValid)
			val.On("ValidateType", tc.previousEvent.Dest).Return(tc.prevEventIsValid)
			tep := TimeElapsedParser{
				initialValidator: val,
			}

			res, err := tep.checkLatestInitialEvent(tc.event, tc.previousEvent, tc.latestBootTime)
			assert.Equal(tc.expectedEvent, res)
			if tc.expectedErr != nil {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			} else {
				assert.Equal(tc.expectedErr, err)
			}
		})
	}
}
func TestGetWRPInfo(t *testing.T) {
	testRegex := eventRegex
	tests := []struct {
		description      string
		msg              wrp.Message
		expectedBootTime int64
		expectedDeviceID string
		expectedErr      error
	}{
		{
			description: "Success",
			msg: wrp.Message{
				Destination: "event:device-status/mac:112233445566/fully-manageable/1615223357",
				Metadata:    map[string]string{bootTimeKey: "60"},
			},
			expectedBootTime: 60,
			expectedDeviceID: "mac:112233445566",
		},
		{
			description: "Event regex no match",
			msg: wrp.Message{
				Destination: "mac:112233445566/fully-manageable/1615223357",
				Metadata:    map[string]string{bootTimeKey: "60"},
			},
			expectedBootTime: 0,
			expectedDeviceID: "",
			expectedErr:      errInvalidDeviceID,
		},
		{
			description: "No boot-time",
			msg: wrp.Message{
				Destination: "event:device-status/mac:112233445566/online",
				Metadata:    map[string]string{},
			},
			expectedBootTime: 0,
			expectedDeviceID: "",
			expectedErr:      errInvalidBootTime,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			bootTime, deviceID, err := getWRPInfo(testRegex, tc.msg)
			assert.Equal(tc.expectedBootTime, bootTime)
			assert.Equal(tc.expectedDeviceID, deviceID)
			if tc.expectedErr != nil {
				assert.True(errors.Is(err, tc.expectedErr),
					fmt.Errorf("error [%v] doesn't contain error [%v] in its err chain",
						err, tc.expectedErr),
				)
			} else {
				assert.Equal(tc.expectedErr, err)
			}
		})
	}
}
