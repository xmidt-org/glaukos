/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package event

import (
	"context"

	"github.com/go-kit/kit/log"
	eventqueue "github.com/xmidt-org/glaukos/event/eventQueue"
	"github.com/xmidt-org/glaukos/event/parsing"
	"go.uber.org/fx"
)

// Provide bundles everything needed for setting up the subscribe endpoint
// together for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		parsing.Provide(),
		eventqueue.ProvideMetrics(),
		fx.Provide(
			func(f func(context.Context) log.Logger) GetLoggerFunc {
				return f
			},
			NewEndpoints,
			NewHandlers,
		),
	)
}
