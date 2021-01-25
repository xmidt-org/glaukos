package queue

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/webpa-common/basculechecks"
	"github.com/xmidt-org/webpa-common/logging"
	"github.com/xmidt-org/webpa-common/semaphore"
	"github.com/xmidt-org/wrp-go/v3"
	"go.uber.org/fx"
)

var (
	defaultLogger = log.NewNopLogger()
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
	queue   chan wrp.Message
	workers semaphore.Interface
	wg      sync.WaitGroup
	logger  log.Logger
	config  Config
	parsers parsing.ParsersIn
	metrics QueueMetricsIn
}

func newEventQueue(config Config, parsers parsing.ParsersIn, metrics QueueMetricsIn, logger log.Logger) (*EventQueue, error) {
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

// ProvideEventQueue creates a new queue and appends the start and stop functions to the uber/fx lifecycle.
func ProvideEventQueue(config Config, lc fx.Lifecycle, parsers parsing.ParsersIn, metrics QueueMetricsIn, logger log.Logger) (*EventQueue, error) {
	e, err := newEventQueue(config, parsers, metrics, logger)

	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(context context.Context) error {
			e.Start()
			return nil
		},
		OnStop: func(context context.Context) error {
			e.Stop()
			return nil
		},
	})

	return e, nil
}

func (e *EventQueue) Start() {
	e.wg.Add(1)
	go e.ParseEvents()
}

func (e *EventQueue) Stop() {
	close(e.queue)
	e.wg.Wait()
}

// Queue attempts to add a message to the queue and returns an error if the queue is full
func (e *EventQueue) Queue(message wrp.Message) (err error) {
	select {
	case e.queue <- message:
		if e.metrics.EventsQueueDepth != nil {
			e.metrics.EventsQueueDepth.Add(1.0)
		}
	default:
		if e.metrics.DroppedEventsCount != nil {
			e.metrics.DroppedEventsCount.With(reasonLabel, queueFullReason).Add(1.0)
		}
		err = NewErrorCode(http.StatusTooManyRequests, errors.New("queue full"))
	}

	return
}

// ParseEvents goes through the queue and calls ParseEvent on each event in the queue
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
