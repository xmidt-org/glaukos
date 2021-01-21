package event

import (
	"errors"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewEventParser(t *testing.T) {
	mockMetadataParser := new(mockParser)
	mockBootTimeCalc := new(mockParser)
	emptyMetrics := QueueMetricsIn{}
	tests := []struct {
		description        string
		config             QueueConfig
		logger             log.Logger
		parsers            ParsersIn
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
			parsers: ParsersIn{
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
				parsers: ParsersIn{
					BootTimeParser: mockBootTimeCalc,
					MetadataParser: mockMetadataParser,
				},
				metrics: emptyMetrics,
			},
		},
		{
			description: "Success with defaults",
			parsers: ParsersIn{
				BootTimeParser: mockBootTimeCalc,
				MetadataParser: mockMetadataParser,
			},
			expectedEventQueue: &EventQueue{
				logger: log.NewNopLogger(),
				config: QueueConfig{
					QueueSize:  defaultMinQueueSize,
					MaxWorkers: defaultMinMaxWorkers,
				},
				parsers: ParsersIn{
					BootTimeParser: mockBootTimeCalc,
					MetadataParser: mockMetadataParser,
				},
			},
		},
		{
			description: "No boot time parser",
			parsers: ParsersIn{
				MetadataParser: mockMetadataParser,
			},
			expectedErr: errors.New("No boot time parser"),
		},
		{
			description: "No metadata parser",
			parsers: ParsersIn{
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
	// msg := wrp.Message{
	// 	Source:          "test source",
	// 	Destination:     "device-status/mac:some_random_mac_address/an-event/some_timestamp",
	// 	Type:            wrp.SimpleEventMessageType,
	// 	PartnerIDs:      []string{"test1", "test2"},
	// 	TransactionUUID: "transaction test uuid",
	// 	Payload:         []byte(`{"ts":"2019-02-13T21:19:02.614191735Z"}`),
	// 	Metadata:        map[string]string{"testkey": "testvalue"},
	// }

	mockMetadataParser := new(mockParser)
	mockBootTimeCalc := new(mockParser)

	mockMetadataParser.On("Parse", mock.Anything).Return(nil)
	mockBootTimeCalc.On("Parse", mock.Anything).Return(nil)
}
