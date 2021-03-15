package queue

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/glaukos/message"
	"go.uber.org/fx"
)

// Queue is the type that processes incoming events
type Queue interface {
	Queue(message.Event) error
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
