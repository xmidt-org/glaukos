/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package eventmetrics

import (
	"context"
	"time"

	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/glaukos/message/validation"

	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/glaukos/eventmetrics/parsers"
	"github.com/xmidt-org/glaukos/eventmetrics/queue"
	"go.uber.org/fx"
)

// Config configures things related to the parsing of events for metrics
type Config struct {
	BirthdateValidFrom time.Duration
	BirthdateValidTo   time.Duration
}

// Provide bundles everything needed for setting up the subscribe endpoint
// together for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		parsers.Provide(),
		queue.Provide(),
		queue.ProvideMetrics(),
		fx.Provide(
			arrange.UnmarshalKey("eventMetrics", Config{}),
			func(f func(context.Context) log.Logger) GetLoggerFunc {
				return f
			},
			// TimeValidator used to validate birthdate of incoming events in NewEndpoints
			func(config Config) validation.TimeValidation {
				return validation.TimeValidator{
					ValidFrom: config.BirthdateValidFrom,
					ValidTo:   config.BirthdateValidTo,
					Current:   time.Now,
				}
			},
			NewEndpoints,
			NewHandlers,
		),
	)
}
