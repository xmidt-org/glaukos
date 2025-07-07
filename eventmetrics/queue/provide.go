// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package queue

import (
	"context"

	"github.com/xmidt-org/arrange"
	"go.uber.org/fx"
	"go.uber.org/zap"
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
		func(in TimeTrackIn) TimeTracker {
			return &timeTracker{
				TimeInMemory: in.TimeInMemory,
			}
		},
		func(config Config, lc fx.Lifecycle, parsersIn ParsersIn, metrics Measures, tracker TimeTracker, logger *zap.Logger) (Queue, error) {
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
