package queue

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/themis/xlog/xlogtest"
	"github.com/xmidt-org/touchstone/touchtest"
	"github.com/xmidt-org/webpa-common/semaphore"
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
	numEvents := 3.0
	queueSize := 5

	if queueSize < int(numEvents) {
		queueSize = int(numEvents)
	}

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
		EventsQueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "testEventsGauge",
			Help: "testEventsGauge",
		}),
	}

	queue := EventQueue{
		parsers:     parsers,
		logger:      xlogtest.New(t),
		workers:     semaphore.New(2),
		metrics:     metrics,
		timeTracker: mockTimeTracker,
		queue:       make(chan EventWithTime, queueSize),
	}

	queue.wg.Add(1)
	metrics.EventsQueueDepth.Set(numEvents)
	go func() {
		for i := 0; i < int(numEvents); i++ {
			queue.queue <- EventWithTime{BeginTime: now, Event: event}
		}
		close(queue.queue)
	}()
	queue.ParseEvents()
	queue.wg.Wait()
	assert.Equal(t, 0.0, testutil.ToFloat64(metrics.EventsQueueDepth))
}

func TestParseEvent(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
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
				EventsCount: prometheus.NewCounterVec(prometheus.CounterOpts{
					Name: "testEventsCount",
					Help: "testEventsCount",
				}, []string{partnerIDLabel, eventDestLabel}),
			},
			event:        EventWithTime{Event: event, BeginTime: now},
			expectedType: "an-event",
		},
		{
			description:         "Bad destination event",
			expectedEventsCount: 1,
			metrics: Measures{
				EventsCount: prometheus.NewCounterVec(prometheus.CounterOpts{
					Name: "testEventsCount",
					Help: "testEventsCount",
				}, []string{partnerIDLabel, eventDestLabel}),
			},
			event:        EventWithTime{Event: badDestEvent, BeginTime: now},
			expectedType: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			expectedRegistry := prometheus.NewPedanticRegistry()
			expectedCount := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "testEventsCount",
				Help: "testEventsCount",
			}, []string{partnerIDLabel, eventDestLabel})
			expectedRegistry.Register(expectedCount)
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
			expectedCount.WithLabelValues("test1", tc.expectedType).Inc()
			testAssert := touchtest.New(t)
			testAssert.Expect(expectedRegistry)
			if tc.metrics.EventsCount != nil {
				assert.True(t, testAssert.CollectAndCompare(tc.metrics.EventsCount))
			}

			for _, parser := range mockParsers {
				parser.AssertCalled(t, "Parse", tc.event.Event)
			}
			mockTimeTracker.AssertExpectations(t)
		})
	}
}

func TestQueue(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	tests := []struct {
		description          string
		errorExpected        error
		queueSize            int
		numEvents            int
		expectedQueueDepth   float64
		expectedDroppedCount float64
	}{
		{
			description:        "Queue not filled",
			queueSize:          10,
			numEvents:          7,
			expectedQueueDepth: 7,
		},
		{
			description:          "Queue overflow",
			queueSize:            10,
			numEvents:            12,
			expectedQueueDepth:   10,
			expectedDroppedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			expectedDepth := prometheus.NewGauge(prometheus.GaugeOpts{
				Name: "testQueueDepth",
				Help: "testQueueDepth",
			})
			expectedDroppedCount := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "testEventsCount",
				Help: "testEventsCount",
			}, []string{reasonLabel})
			metrics := Measures{
				EventsQueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
					Name: "testQueueDepth",
					Help: "testQueueDepth",
				}),
				DroppedEventsCount: prometheus.NewCounterVec(prometheus.CounterOpts{
					Name: "testEventsCount",
					Help: "testEventsCount",
				}, []string{reasonLabel}),
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

			if tc.expectedQueueDepth > 0 {
				expectedDepth.Set(tc.expectedQueueDepth)
			}

			if tc.expectedDroppedCount > 0 {
				expectedDroppedCount.WithLabelValues(queueFullReason).Add(tc.expectedDroppedCount)
			}

			testAssert := touchtest.New(t)
			expectedRegistry := prometheus.NewPedanticRegistry()
			actualRegistry := prometheus.NewPedanticRegistry()
			expectedRegistry.Register(expectedDepth)
			expectedRegistry.Register(expectedDroppedCount)
			actualRegistry.Register(metrics.EventsQueueDepth)
			actualRegistry.Register(metrics.DroppedEventsCount)

			testAssert.Expect(expectedRegistry)
			assert.True(testAssert.GatherAndCompare(actualRegistry))
			mockTimeTracker.AssertExpectations(t)
		})
	}
}
