package eventqueue

import (
	"errors"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/webpa-common/logging"
	"github.com/xmidt-org/webpa-common/semaphore"
	"github.com/xmidt-org/webpa-common/xmetrics"
	"github.com/xmidt-org/webpa-common/xmetrics/xmetricstest"
	"github.com/xmidt-org/wrp-go/v3"
)

func TestNewEventParser(t *testing.T) {
	mockMetadataParser := new(parsing.MockParser)
	mockBootTimeCalc := new(parsing.MockParser)
	emptyMetrics := QueueMetricsIn{}
	tests := []struct {
		description        string
		config             QueueConfig
		logger             log.Logger
		parsers            parsing.ParsersIn
		metrics            QueueMetricsIn
		expectedEventQueue *EventQueue
		expectedErr        error
	}{
		{
			description: "Custom config success",
			logger:      log.NewJSONLogger(os.Stdout),
			config: QueueConfig{
				QueueSize:  100,
				MaxWorkers: 10,
			},
			parsers: parsing.ParsersIn{
				BootTimeParser: mockBootTimeCalc,
				MetadataParser: mockMetadataParser,
			},
			metrics: emptyMetrics,
			expectedEventQueue: &EventQueue{
				logger: log.NewJSONLogger(os.Stdout),
				config: QueueConfig{
					QueueSize:  100,
					MaxWorkers: 10,
				},
				parsers: parsing.ParsersIn{
					BootTimeParser: mockBootTimeCalc,
					MetadataParser: mockMetadataParser,
				},
				metrics: emptyMetrics,
			},
		},
		{
			description: "Success with defaults",
			parsers: parsing.ParsersIn{
				BootTimeParser: mockBootTimeCalc,
				MetadataParser: mockMetadataParser,
			},
			expectedEventQueue: &EventQueue{
				logger: log.NewNopLogger(),
				config: QueueConfig{
					QueueSize:  defaultMinQueueSize,
					MaxWorkers: defaultMinMaxWorkers,
				},
				parsers: parsing.ParsersIn{
					BootTimeParser: mockBootTimeCalc,
					MetadataParser: mockMetadataParser,
				},
			},
		},
		{
			description: "No boot time parser",
			parsers: parsing.ParsersIn{
				MetadataParser: mockMetadataParser,
			},
			expectedErr: errors.New("No boot time parser"),
		},
		{
			description: "No metadata parser",
			parsers: parsing.ParsersIn{
				BootTimeParser: mockBootTimeCalc,
			},
			expectedErr: errors.New("No metadata parser"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			queue, err := NewEventQueue(tc.config, tc.parsers, QueueMetricsIn{}, tc.logger)

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
		metrics             QueueMetricsIn
	}{
		{
			description:         "Without metrics",
			expectedEventsCount: 0,
			metrics:             QueueMetricsIn{},
		},
		{
			description:         "With metrics",
			expectedEventsCount: 1,
			metrics: QueueMetricsIn{
				EventsCount: p.NewCounter("depth"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			mockMetadataParser := new(parsing.MockParser)
			mockBootTimeCalc := new(parsing.MockParser)

			mockMetadataParser.On("Parse", mock.Anything).Return(nil).Once()
			mockBootTimeCalc.On("Parse", mock.Anything).Return(nil).Once()

			parsers := parsing.ParsersIn{
				MetadataParser: mockMetadataParser,
				BootTimeParser: mockBootTimeCalc,
			}

			queue := EventQueue{
				config: QueueConfig{
					MaxWorkers: 10,
					QueueSize:  10,
				},
				parsers: parsers,
				logger:  logging.NewTestLogger(nil, t),
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
			metrics := QueueMetricsIn{
				EventsQueueDepth:   p.NewGauge("depth"),
				DroppedEventsCount: p.NewCounter("dropped"),
			}

			q := EventQueue{
				logger:  logging.NewTestLogger(nil, t),
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
