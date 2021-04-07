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
	"context"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/themis/xmetrics"
	"go.uber.org/fx"
)

// Queue is the type that processes incoming events
type Queue interface {
	Queue(EventWithTime) error
}

// ParsersIn brings together all of the different types of parsers that glaukos uses.
type ParsersIn struct {
	fx.In
	Parsers []Parser `group:"parsers"`
}

// Provide creates an uber/fx option and appends the queue start and stop into the fx lifecycle.
func Provide() fx.Option {
	return fx.Provide(
		arrange.UnmarshalKey("queue", Config{}),
		newTimeTracker,
		func(config Config, lc fx.Lifecycle, parsersIn ParsersIn, metrics Measures, tracker TimeTracker, logger log.Logger) (Queue, error) {
			e, err := newEventQueue(config, parsersIn.Parsers, metrics, tracker, logger)

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
		},
	)
}

func newTimeTracker(f xmetrics.Factory) (TimeTracker, error) {
	opts := prometheus.HistogramOpts{
		Name: "time_in_memory",
		Help: "the amount of time an event stays in memory",
	}

	histogram, err := f.NewHistogram(opts, []string{})

	if err != nil {
		return nil, err
	}

	return &timeTracker{
		TimeInMemory: histogram,
	}, nil
}
