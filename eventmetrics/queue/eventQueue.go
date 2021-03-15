package queue

import (
	"errors"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/xmidt-org/glaukos/message"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/webpa-common/basculechecks"
	"github.com/xmidt-org/webpa-common/semaphore"
)

var (
	defaultLogger = log.NewNopLogger()

	errNoParsers = errors.New("No parsers")
)

const (
	defaultMinMaxWorkers = 5
	defaultMinQueueSize  = 5
)

// QueueConfig configures the glaukos queue used to parse incoming events from Caduceus
type Config struct {
	QueueSize  int
	MaxWorkers int
}

type EventQueue struct {
	queue   chan message.Event
	workers semaphore.Interface
	wg      sync.WaitGroup
	logger  log.Logger
	config  Config
	parsers []Parser
	metrics Measures
}

// Parser is the interface that all glaukos parsers must implement.
type Parser interface {
	Parse(message.Event) error
}

func newEventQueue(config Config, parsers []Parser, metrics Measures, logger log.Logger) (*EventQueue, error) {
	if len(parsers) == 0 {
		return nil, errNoParsers
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

	queue := make(chan message.Event, config.QueueSize)
	workers := semaphore.New(config.MaxWorkers)

	e := EventQueue{
		config:  config,
		queue:   queue,
		logger:  logger,
		workers: workers,
		parsers: parsers,
		metrics: metrics,
	}

	return &e, nil
}

func (e *EventQueue) Start() {
	e.wg.Add(1)
	go e.ParseEvents()
}

func (e *EventQueue) Stop() {
	close(e.queue)
	e.wg.Wait()
}

// Queue attempts to add a message to the queue and returns an error if the queue is full.
func (e *EventQueue) Queue(event message.Event) (err error) {
	select {
	case e.queue <- event:
		if e.metrics.EventsQueueDepth != nil {
			e.metrics.EventsQueueDepth.Add(1.0)
		}
	default:
		if e.metrics.DroppedEventsCount != nil {
			e.metrics.DroppedEventsCount.With(reasonLabel, queueFullReason).Add(1.0)
		}
		err = TooManyRequestsErr{Message: "Queue Full"}
	}

	return
}

// ParseEvents goes through the queue and calls ParseEvent on each event in the queue.
func (e *EventQueue) ParseEvents() {
	defer e.wg.Done()
	for event := range e.queue {
		if e.metrics.EventsQueueDepth != nil {
			e.metrics.EventsQueueDepth.Add(-1.0)
		}
		e.workers.Acquire()
		go e.ParseEvent(event)
	}
}

// ParseEvent parses the metadata and boot-time of each event and generates metrics.
func (e *EventQueue) ParseEvent(event message.Event) {
	defer e.workers.Release()
	if e.metrics.EventsCount != nil {
		partnerID := basculechecks.DeterminePartnerMetric(event.PartnerIDs)
		e.metrics.EventsCount.With(partnerIDLabel, partnerID).Add(1.0)
	}

	for _, p := range e.parsers {
		if err := p.Parse(event); err != nil {
			level.Error(e.logger).Log(xlog.ErrorKey(), err, xlog.MessageKey(), "failed to parse")
		}
	}
}