package queue

import (
	"errors"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/themis/xlog/xlogtest"
	"github.com/xmidt-org/webpa-common/semaphore"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestNewEventParser(t *testing.T) {
	mockMetadataParser := new(MockParser)
	mockBootTimeCalc := new(MockParser)
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
			parsers: []Parser{mockBootTimeCalc, mockMetadataParser},
			metrics: emptyMetrics,
			expectedEventQueue: &EventQueue{
				logger: log.NewJSONLogger(os.Stdout),
				config: Config{
					QueueSize:  100,
					MaxWorkers: 10,
				},
				parsers: []Parser{mockBootTimeCalc, mockMetadataParser},
				metrics: emptyMetrics,
			},
		},
		{
			description: "Success with defaults",
			parsers:     []Parser{mockBootTimeCalc, mockMetadataParser},
			expectedEventQueue: &EventQueue{
				logger: log.NewNopLogger(),
				config: Config{
					QueueSize:  defaultMinQueueSize,
					MaxWorkers: defaultMinMaxWorkers,
				},
				parsers: []Parser{mockBootTimeCalc, mockMetadataParser},
			},
		},
		{
			description: "No parsers",
			parsers:     []Parser{},
			expectedErr: errors.New("No parsers"),
		},
		{
			description: "Nil parser",
			parsers:     nil,
			expectedErr: errors.New("No parsers"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			queue, err := newEventQueue(tc.config, tc.parsers, Measures{}, tc.logger)

			assert.Equal(tc.expectedErr, err)

			if tc.expectedErr == nil || err == nil {
				assert.NotNil(queue.queue)
				assert.NotNil(queue.workers)
				tc.expectedEventQueue.queue = queue.queue
				tc.expectedEventQueue.workers = queue.workers

			}

			assert.Equal(tc.expectedEventQueue, queue)

		})
	}
}

func TestParseEvent(t *testing.T) {
	p := xmetricstest.NewProvider(&xmetrics.Options{})
	msg := wrp.Message{
		Source:          "test source",
		Destination:     "device-status/mac:some_address/an-event/some_timestamp",
		Type:            wrp.SimpleEventMessageType,
		PartnerIDs:      []string{"test1"},
		TransactionUUID: "transaction test uuid",
		Payload:         []byte(`{"ts":"2019-02-13T21:19:02.614191735Z"}`),
		Metadata:        map[string]string{"testkey": "testvalue"},
	}

	tests := []struct {
		description         string
		expectedEventsCount float64
		metrics             Measures
	}{
		{
			description:         "Without metrics",
			expectedEventsCount: 0,
			metrics:             Measures{},
		},
		{
			description:         "With metrics",
			expectedEventsCount: 1,
			metrics: Measures{
				EventsCount: p.NewCounter("depth"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			mockMetadataParser := new(MockParser)
			mockBootTimeCalc := new(MockParser)

			mockMetadataParser.On("Parse", mock.Anything).Return(nil).Once()
			mockBootTimeCalc.On("Parse", mock.Anything).Return(nil).Once()

			parsers := []Parser{mockBootTimeCalc, mockMetadataParser}

			queue := EventQueue{
				config: Config{
					MaxWorkers: 10,
					QueueSize:  10,
				},
				parsers: parsers,
				logger:  xlogtest.New(t),
				workers: semaphore.New(2),
				metrics: tc.metrics,
			}

			queue.workers.Acquire()
			queue.ParseEvent(msg)

			mockMetadataParser.AssertCalled(t, "Parse", msg)
			mockBootTimeCalc.AssertCalled(t, "Parse", msg)
			p.Assert(t, "depth", partnerIDLabel, "test1")(xmetricstest.Value(tc.expectedEventsCount))

		})
	}
}

func TestQueue(t *testing.T) {
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

			q := EventQueue{
				logger:  xlogtest.New(t),
				workers: semaphore.New(2),
				metrics: metrics,
				queue:   make(chan wrp.Message, tc.queueSize),
			}

			for i := 0; i < tc.numEvents; i++ {
				q.Queue(wrp.Message{})
			}

			p.Assert(t, "depth")(xmetricstest.Value(tc.eventsMetricCount))
			p.Assert(t, "dropped", reasonLabel, queueFullReason)(xmetricstest.Value(tc.droppedMetricCount))
		})
	}
}
