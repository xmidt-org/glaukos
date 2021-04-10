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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/touchstone"
	"go.uber.org/fx"
)

const (
	partnerIDLabel  = "partner_id"
	reasonLabel     = "reason"
	queueFullReason = "queue_full"
	eventDestLabel  = "event_destination"
)

// Measures contains the various queue-related metrics.
type Measures struct {
	fx.In
	EventsQueueDepth   prometheus.Gauge       `name:"events_queue_depth"`
	EventsCount        *prometheus.CounterVec `name:"events_count"`
	DroppedEventsCount *prometheus.CounterVec `name:"dropped_events_count"`
}

type TimeTrackIn struct {
	fx.In
	TimeInMemory prometheus.Observer `name:"time_in_memory"`
}

// ProvideMetrics builds the queue-related metrics and makes them available to the container.
func ProvideMetrics() fx.Option {
	return fx.Options(
		touchstone.Gauge(
			prometheus.GaugeOpts{
				Name: "events_queue_depth",
				Help: "The depth of the event queue",
			},
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "events_count",
				Help: "Details of incoming events",
			},
			partnerIDLabel,
			eventDestLabel,
		),
		touchstone.CounterVec(
			prometheus.CounterOpts{
				Name: "dropped_events_count",
				Help: "The total number of events dropped",
			},
			reasonLabel,
		),
		touchstone.Histogram(
			prometheus.HistogramOpts{
				Name:    "time_in_memory",
				Help:    "The amount of time an event stays in memory",
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
		),
	)
}

type timeTracker struct {
	TimeInMemory prometheus.Observer
}

func (t *timeTracker) TrackTime(length time.Duration) {
	t.TimeInMemory.Observe(length.Seconds())
}
