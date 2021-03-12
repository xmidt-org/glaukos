/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package eventmetrics

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/glaukos/event/parsers"
	"github.com/xmidt-org/glaukos/event/queue"
	"go.uber.org/fx"
)

// Provide bundles everything needed for setting up the subscribe endpoint
// together for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		parsers.Provide(),
		queue.Provide(),
		queue.ProvideMetrics(),
		fx.Provide(
			func(f func(context.Context) log.Logger) GetLoggerFunc {
				return f
			},
			NewEndpoints,
			NewHandlers,
		),
	)
}
