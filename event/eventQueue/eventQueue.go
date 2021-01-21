package eventqueue

import (
	"errors"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/webpa-common/basculechecks"
	"github.com/xmidt-org/webpa-common/logging"
	"github.com/xmidt-org/webpa-common/semaphore"
	"github.com/xmidt-org/wrp-go/v3"
)

var (
	defaultLogger = log.NewNopLogger()
)

const (
	defaultMinMaxWorkers = 5
	defaultMinQueueSize  = 5
)

// QueueConfig configures the glaukos queue used to parse incoming events from Caduceus
type QueueConfig struct {
	QueueSize  int
	MaxWorkers int
}

type EventQueue struct {
	queue   chan wrp.Message
	workers semaphore.Interface
	wg      sync.WaitGroup
	logger  log.Logger
	config  QueueConfig
	parsers parsing.ParsersIn
	metrics QueueMetricsIn
}

func NewEventQueue(config QueueConfig, parsers parsing.ParsersIn, metrics QueueMetricsIn, logger log.Logger) (*EventQueue, error) {
	if parsers.BootTimeParser == nil {
		return nil, errors.New("No boot time parser")
	}

	if parsers.MetadataParser == nil {
		return nil, errors.New("No metadata parser")
	}

	if config.MaxWorkers < defaultMinMaxWorkers {
		config.MaxWorkers = defaultMinMaxWorkers
	}

	if config.QueueSize < defaultMinQueueSize {
		config.QueueSize = defaultMinQueueSize
	}

	if logger == nil {
		logger = defaultLogger
	}

	queue := make(chan wrp.Message, config.QueueSize)
	workers := semaphore.New(config.MaxWorkers)

	r := EventQueue{
		config:  config,
		queue:   queue,
		logger:  logger,
		workers: workers,
		parsers: parsers,
		metrics: metrics,
	}

	return &r, nil
}

func (e *EventQueue) Start() {
	e.wg.Add(1)
	go e.ParseEvents()
}

// Queue attempts to add a message to the queue and returns an error if the queue is full
func (e *EventQueue) Queue(message wrp.Message) (err error) {
	select {
	case e.queue <- message:
		if e.metrics.EventQueue != nil {
			e.metrics.EventQueue.Add(1.0)
		}
		logging.Debug(e.logger).Log(logging.MessageKey(), "queued message")
	default:
		logging.Error(e.logger).Log(logging.MessageKey(), "queue full")
		err = QueueFullError{Message: "Queue Full"}
	}

	return
}

func (e *EventQueue) Stop() {
	close(e.queue)
	e.wg.Wait()
}

// ParseEvents goes through the queue and calls ParseEvent on each event in the queue
func (e *EventQueue) ParseEvents() {
	defer e.wg.Done()
	for event := range e.queue {
		if e.metrics.EventQueue != nil {
			e.metrics.EventQueue.Add(-1.0)
		}
		e.workers.Acquire()
		go e.ParseEvent(event)
	}
}

// ParseEvent parses the metadata and boot-time of each event and generates metrics
func (e *EventQueue) ParseEvent(message wrp.Message) {
	defer e.workers.Release()
	if e.metrics.EventsCount != nil {
		partnerID := basculechecks.DeterminePartnerMetric(message.PartnerIDs)
		e.metrics.EventsCount.With(partnerIDLabel, partnerID).Add(1.0)
	}

	err := e.parsers.MetadataParser.Parse(message)
	if err != nil {
		logging.Error(e.logger).Log(logging.MessageKey(), "failed to do metadata parse", logging.ErrorKey(), err)
	}
	err = e.parsers.BootTimeParser.Parse(message)
	if err != nil {
		logging.Error(e.logger).Log(logging.MessageKey(), "failed to do boot time parse", logging.ErrorKey(), err)
	}

}
