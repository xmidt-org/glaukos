/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package event

import (
	"context"

	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/bascule/acquire"
	"github.com/xmidt-org/webpa-common/logging"
	"go.uber.org/fx"
)

type CodexConfig struct {
	Address string
	Auth    AuthAcquirerConfig
}
type AuthAcquirerConfig struct {
	JWT   acquire.RemoteBearerTokenAcquirerOptions
	Basic string
}

// Provide bundles everything needed for setting up the subscribe endpoint
// together for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideMetrics(),
		fx.Provide(
			func(f func(context.Context) log.Logger) GetLoggerFunc {
				return f
			},
			func(in MetricsIn) MetadataParser {
				return MetadataParser{
					MetadataFields: in.MetadataFields,
				}
			},
			arrange.UnmarshalKey("codex", CodexConfig{}),
			func(logger log.Logger, metricsIn MetricsIn, codexConfig CodexConfig) BootTimeCalc {
				var codexAuth acquire.Acquirer = &acquire.DefaultAcquirer{}
				jwtAuth, err := acquire.NewRemoteBearerTokenAcquirer(codexConfig.Auth.JWT)
				if err == nil {
					codexAuth = jwtAuth
					logging.Debug(logger).Log(logging.MessageKey(), "using jwt")
				} else if codexConfig.Auth.Basic != "" {
					codexAuth, err = acquire.NewFixedAuthAcquirer(codexConfig.Auth.Basic)
					logging.Debug(logger).Log(logging.MessageKey(), "using basic")
				} else {
					logging.Error(logger).Log(logging.MessageKey(), "failed to create acquirer")
				}

				return BootTimeCalc{
					BootTimeHistogram: metricsIn.BootTimeHistogram,
					Logger:            logger,
					Address:           codexConfig.Address,
					Auth:              codexAuth,
				}
			},
			NewEndpoints,
			NewHandlers,
		),
	)
}
