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
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/interpreter"
	"go.uber.org/fx"
)

// Queue is the type that processes incoming events
type Queue interface {
	Queue(interpreter.Event) error
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
		func(config Config, lc fx.Lifecycle, parsersIn ParsersIn, metrics Measures, logger log.Logger) (Queue, error) {
			e, err := newEventQueue(config, parsersIn.Parsers, metrics, logger)

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
