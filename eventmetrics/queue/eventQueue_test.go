package queue

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/themis/xlog/xlogtest"
	"github.com/xmidt-org/webpa-common/semaphore"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestNewEventParser(t *testing.T) {
	mockParser1 := new(mockParser)
	mockParser2 := new(mockParser)
	mockTimeTracker := new(mockTimeTracker)
	emptyMetrics := Measures{}
	tests := []struct {
		description        string
		config             Config
		logger             log.Logger
		parsers            []Parser
		metrics            Measures
		expectedEventQueue *EventQueue
		expectedErr        error
	}{
		{
			description: "Custom config success",
			logger:      log.NewJSONLogger(os.Stdout),
			config: Config{
				QueueSize:  100,
				MaxWorkers: 10,
			},
			parsers: []Parser{mockParser1, mockParser2},
			metrics: emptyMetrics,
			expectedEventQueue: &EventQueue{
				logger: log.NewJSONLogger(os.Stdout),
				config: Config{
					QueueSize:  100,
					MaxWorkers: 10,
				},
				parsers: []Parser{mockParser1, mockParser2},
				metrics: emptyMetrics,
			},
		},
		{
			description: "Success with defaults",
			parsers:     []Parser{mockParser1, mockParser2},
			expectedEventQueue: &EventQueue{
				logger: log.NewNopLogger(),
				config: Config{
					QueueSize:  defaultMinQueueSize,
					MaxWorkers: defaultMaxWorkers,
				},
				parsers: []Parser{mockParser1, mockParser2},
			},
		},
		{
			description: "No parsers",
			parsers:     []Parser{},
			expectedErr: errNoParsers,
		},
		{
			description: "Nil parser",
			parsers:     nil,
			expectedErr: errNoParsers,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			queue, err := newEventQueue(tc.config, tc.parsers, Measures{}, mockTimeTracker, tc.logger)

			if tc.expectedErr != nil || err != nil {
				assert.True(errors.Is(err, tc.expectedErr))
			}

			if tc.expectedErr == nil || err == nil {
				assert.NotNil(queue.queue)
				assert.NotNil(queue.workers)
				assert.Equal(mockTimeTracker, queue.timeTracker)
				tc.expectedEventQueue.queue = queue.queue
				tc.expectedEventQueue.workers = queue.workers
				tc.expectedEventQueue.timeTracker = queue.timeTracker

			}

			assert.Equal(tc.expectedEventQueue, queue)

		})
	}
}

func TestStop(t *testing.T) {
	queue := EventQueue{
		queue: make(chan EventWithTime, 5),
	}

	queue.Stop()
	_, more := <-queue.queue
	assert.False(t, more)
}

func TestParseEvents(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	p := xmetricstest.NewProvider(&xmetrics.Options{})
	event := interpreter.Event{
		Source:          "test source",
		Destination:     "event:device-status/mac:some_address/an-event/some_timestamp",
		MsgType:         int(wrp.SimpleEventMessageType),
		PartnerIDs:      []string{"test1"},
		TransactionUUID: "transaction test uuid",
		Payload:         `{"ts":"2019-02-13T21:19:02.614191735Z"}`,
		Metadata:        map[string]string{"testkey": "testvalue"},
	}

	mockParsers := []*mockParser{new(mockParser), new(mockParser)}
	parsers := make([]Parser, 0, len(mockParsers))
	for _, parser := range mockParsers {
		parser.On("Parse", mock.Anything).Return(nil)
		parsers = append(parsers, parser)
	}

	mockTimeTracker := new(mockTimeTracker)
	mockTimeTracker.On("TrackTime", mock.Anything)
	metrics := Measures{
		EventsCount:      p.NewCounter("events_count"),
		EventsQueueDepth: p.NewGauge("depth"),
	}

	queue := EventQueue{
		parsers:     parsers,
		logger:      xlogtest.New(t),
		workers:     semaphore.New(2),
		metrics:     metrics,
		timeTracker: mockTimeTracker,
		queue:       make(chan EventWithTime, 5),
	}

	queue.wg.Add(1)
	go func() {
		for i := 0; i < 3; i++ {
			queue.queue <- EventWithTime{BeginTime: now, Event: event}
		}
		close(queue.queue)
	}()
	queue.ParseEvents()
	queue.wg.Wait()
	p.Assert(t, "depth")(xmetricstest.Value(-3.0))
}

func TestParseEvent(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	p := xmetricstest.NewProvider(&xmetrics.Options{})
	event := interpreter.Event{
		Source:          "test source",
		Destination:     "event:device-status/mac:some_address/an-event/some_timestamp",
		MsgType:         int(wrp.SimpleEventMessageType),
		PartnerIDs:      []string{"test1"},
		TransactionUUID: "transaction test uuid",
		Payload:         `{"ts":"2019-02-13T21:19:02.614191735Z"}`,
		Metadata:        map[string]string{"testkey": "testvalue"},
	}

	badDestEvent := interpreter.Event{
		Source:          "test source",
		Destination:     "some-event",
		MsgType:         int(wrp.SimpleEventMessageType),
		PartnerIDs:      []string{"test1"},
		TransactionUUID: "transaction test uuid",
		Payload:         `{"ts":"2019-02-13T21:19:02.614191735Z"}`,
		Metadata:        map[string]string{"testkey": "testvalue"},
	}

	tests := []struct {
		description         string
		expectedEventsCount float64
		metrics             Measures
		event               EventWithTime
		expectedType        string
	}{
		{
			description:         "Without metrics",
			expectedEventsCount: 0,
			metrics:             Measures{},
			event:               EventWithTime{Event: event, BeginTime: now},
			expectedType:        "an-event",
		},
		{
			description:         "With metrics",
			expectedEventsCount: 1,
			metrics: Measures{
				EventsCount: p.NewCounter("depth"),
			},
			event:        EventWithTime{Event: event, BeginTime: now},
			expectedType: "an-event",
		},
		{
			description:         "Bad destination event",
			expectedEventsCount: 1,
			metrics: Measures{
				EventsCount: p.NewCounter("depth"),
			},
			event:        EventWithTime{Event: badDestEvent, BeginTime: now},
			expectedType: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			mockParsers := []*mockParser{new(mockParser), new(mockParser)}
			parsers := make([]Parser, 0, len(mockParsers))
			for _, parser := range mockParsers {
				parser.On("Parse", mock.Anything).Return(nil).Once()
				parsers = append(parsers, parser)
			}

			mockTimeTracker := new(mockTimeTracker)
			mockTimeTracker.On("TrackTime", mock.Anything).Once()

			queue := EventQueue{
				config: Config{
					MaxWorkers: 10,
					QueueSize:  10,
				},
				parsers:     parsers,
				logger:      xlogtest.New(t),
				workers:     semaphore.New(2),
				metrics:     tc.metrics,
				timeTracker: mockTimeTracker,
			}

			queue.workers.Acquire()
			queue.ParseEvent(tc.event)

			for _, parser := range mockParsers {
				parser.AssertCalled(t, "Parse", tc.event.Event)
			}
			mockTimeTracker.AssertExpectations(t)
			p.Assert(t, "depth", partnerIDLabel, "test1", eventDestLabel, tc.expectedType)(xmetricstest.Value(tc.expectedEventsCount))

		})
	}
}

func TestQueue(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	tests := []struct {
		description        string
		errorExpected      error
		queueSize          int
		numEvents          int
		eventsMetricCount  float64
		droppedMetricCount float64
	}{
		{
			description:       "Queue not filled",
			queueSize:         10,
			numEvents:         7,
			eventsMetricCount: 7,
		},
		{
			description:        "Queue overflow",
			queueSize:          10,
			numEvents:          12,
			eventsMetricCount:  10,
			droppedMetricCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			p := xmetricstest.NewProvider(&xmetrics.Options{})
			metrics := Measures{
				EventsQueueDepth:   p.NewGauge("depth"),
				DroppedEventsCount: p.NewCounter("dropped"),
			}

			mockTimeTracker := new(mockTimeTracker)

			q := EventQueue{
				logger:      xlogtest.New(t),
				workers:     semaphore.New(2),
				metrics:     metrics,
				queue:       make(chan EventWithTime, tc.queueSize),
				timeTracker: mockTimeTracker,
			}

			for i := 0; i < tc.numEvents; i++ {
				if i >= tc.queueSize {
					mockTimeTracker.On("TrackTime", mock.Anything)
				}
				q.Queue(EventWithTime{BeginTime: now})
			}

			p.Assert(t, "depth")(xmetricstest.Value(tc.eventsMetricCount))
			p.Assert(t, "dropped", reasonLabel, queueFullReason)(xmetricstest.Value(tc.droppedMetricCount))
			mockTimeTracker.AssertExpectations(t)
		})
	}
}
