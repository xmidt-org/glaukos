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

	"github.com/go-kit/kit/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/themis/xmetrics"
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
	EventsQueueDepth   metrics.Gauge   `name:"events_queue_depth"`
	EventsCount        metrics.Counter `name:"events_count"`
	DroppedEventsCount metrics.Counter `name:"dropped_events_count"`
}

// ProvideMetrics builds the queue-related metrics and makes them available to the container.
func ProvideMetrics() fx.Option {
	return fx.Provide(
		xmetrics.ProvideGauge(
			prometheus.GaugeOpts{
				Name: "events_queue_depth",
				Help: "The depth of the event queue",
			},
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "events_count",
				Help: "Details of incoming events",
			},
			partnerIDLabel,
			eventDestLabel,
		),
		xmetrics.ProvideCounter(
			prometheus.CounterOpts{
				Name: "dropped_events_count",
				Help: "The total number of events dropped",
			},
			reasonLabel,
		),
	)
}

type timeTracker struct {
	TimeInMemory metrics.Histogram
}

func (t *timeTracker) TrackTime(length time.Duration) {
	t.TimeInMemory.Observe(length.Seconds())
}
