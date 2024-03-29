/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package queue

import (
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/webpa-common/v2/basculechecks"
	"github.com/xmidt-org/webpa-common/v2/semaphore"
)

var (
	defaultLogger = zap.NewNop()

	errNoParsers = errors.New("No parsers")
)

const (
	defaultMaxWorkers   = 5
	defaultMinQueueSize = 5
)

// TimeTracker tracks the time an event is in memory.
type TimeTracker interface {
	TrackTime(time.Duration)
}

// Config configures the glaukos queue used to parse incoming events from Caduceus
type Config struct {
	QueueSize  int
	MaxWorkers int
}

// EventQueue processes incoming events
type EventQueue struct {
	queue       chan EventWithTime
	workers     semaphore.Interface
	wg          sync.WaitGroup
	logger      *zap.Logger
	config      Config
	parsers     []Parser
	metrics     Measures
	timeTracker TimeTracker
}

// Parser is the interface that all glaukos parsers must implement.
type Parser interface {
	Parse(interpreter.Event)
	Name() string
}

// EventWithTime allows for the tracking of how long an event stays in glaukos's memory.
type EventWithTime struct {
	Event     interpreter.Event
	BeginTime time.Time
}

func newEventQueue(config Config, parsers []Parser, metrics Measures, tracker TimeTracker, logger *zap.Logger) (*EventQueue, error) {
	if len(parsers) == 0 {
		return nil, errNoParsers
	}

	if config.MaxWorkers < defaultMaxWorkers {
		config.MaxWorkers = defaultMaxWorkers
	}

	if config.QueueSize < defaultMinQueueSize {
		config.QueueSize = defaultMinQueueSize
	}

	if logger == nil {
		logger = defaultLogger
	}

	queue := make(chan EventWithTime, config.QueueSize)
	workers := semaphore.New(config.MaxWorkers)

	e := EventQueue{
		config:      config,
		queue:       queue,
		logger:      logger,
		workers:     workers,
		parsers:     parsers,
		metrics:     metrics,
		timeTracker: tracker,
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
func (e *EventQueue) Queue(eventWithTime EventWithTime) (err error) {
	select {
	case e.queue <- eventWithTime:
		if e.metrics.EventsQueueDepth != nil {
			e.metrics.EventsQueueDepth.Add(1.0)
		}
	default:
		if e.metrics.DroppedEventsCount != nil {
			e.metrics.DroppedEventsCount.With(prometheus.Labels{reasonLabel: queueFullReason}).Add(1.0)
		}
		e.timeTracker.TrackTime(time.Since(eventWithTime.BeginTime))
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
func (e *EventQueue) ParseEvent(eventWithTime EventWithTime) {
	defer e.workers.Release()
	if e.metrics.EventsCount != nil {
		event := eventWithTime.Event
		partnerID := basculechecks.DeterminePartnerMetric(event.PartnerIDs)
		eventType, err := event.EventType()
		if err != nil {
			e.logger.Error("unable to get event type")
			eventType = "unknown"
		}
		e.metrics.EventsCount.With(prometheus.Labels{partnerIDLabel: partnerID, eventDestLabel: eventType}).Add(1.0)
	}

	for _, p := range e.parsers {
		p.Parse(eventWithTime.Event)
	}

	e.timeTracker.TrackTime(time.Since(eventWithTime.BeginTime))
}
