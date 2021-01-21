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

// CodexConfig determines the auth and address for connecting to the codex cluster
type CodexConfig struct {
	Address string
	Auth    AuthAcquirerConfig
}

type AuthAcquirerConfig struct {
	JWT   acquire.RemoteBearerTokenAcquirerOptions
	Basic string
}

type ParsersOut struct {
	fx.Out
	BootTimeParser parser `name:"bootTimeParser"`
	MetadataParser parser `name:"metadataParser"`
}

// Provide bundles everything needed for setting up the subscribe endpoint
// together for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideEventMetrics(),
		ProvideQueueMetrics(),
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
				codexAuth, err := determineCodexTokenAcquirer(logger, codexConfig)
				if err != nil {
					logging.Error(logger).Log(logging.MessageKey(), "failed to create acquirer", "error", err)
				}

				return BootTimeCalc{
					BootTimeHistogram: metricsIn.BootTimeHistogram,
					Logger:            logger,
					Address:           codexConfig.Address,
					Auth:              codexAuth,
				}
			},
			func(calc BootTimeCalc, mp MetadataParser) ParsersOut {
				return ParsersOut{
					BootTimeParser: calc,
					MetadataParser: mp,
				}
			},
			NewEndpoints,
			NewHandlers,
		),
	)
}

func determineCodexTokenAcquirer(logger log.Logger, config CodexConfig) (acquire.Acquirer, error) {
	defaultAcquirer := &acquire.DefaultAcquirer{}
	jwt := config.Auth.JWT
	if jwt.AuthURL != "" && jwt.Buffer > 0 && jwt.Timeout > 0 {
		logging.Debug(logger).Log(logging.MessageKey(), "using jwt")
		return acquire.NewRemoteBearerTokenAcquirer(jwt)
	}

	if config.Auth.Basic != "" {
		logging.Debug(logger).Log(logging.MessageKey(), "using basic")
		return acquire.NewFixedAuthAcquirer(config.Auth.Basic)
	}

	logging.Error(logger).Log(logging.MessageKey(), "failed to create acquirer")
	return defaultAcquirer, nil

}
