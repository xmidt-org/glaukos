// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package eventmetrics

import (
	"context"
	"time"

	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/interpreter/validation"

	"github.com/xmidt-org/glaukos/eventmetrics/parsers"
	"github.com/xmidt-org/glaukos/eventmetrics/queue"
	"go.uber.org/fx"
	"go.uber.org/zap"
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
			func(f func(context.Context) *zap.Logger) GetLoggerFunc {
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
