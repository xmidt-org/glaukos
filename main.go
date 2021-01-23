/**
 *  Copyright (c) 2020  Comcast Cable Communications Management, LLC
 */

package main

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/InVisionApp/go-health"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/provider"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/arrange/arrangehttp"
	"github.com/xmidt-org/bascule/basculehttp"
	"github.com/xmidt-org/glaukos/event"
	"github.com/xmidt-org/glaukos/event/parsing"
	"github.com/xmidt-org/glaukos/event/queue"
	"github.com/xmidt-org/themis/config"
	"github.com/xmidt-org/themis/xhealth"
	"github.com/xmidt-org/themis/xhttp/xhttpserver"
	"github.com/xmidt-org/themis/xlog"
	"github.com/xmidt-org/themis/xlog/xloghttp"
	"github.com/xmidt-org/themis/xmetrics/xmetricshttp"
	"github.com/xmidt-org/webpa-common/basculechecks"
	"github.com/xmidt-org/webpa-common/basculemetrics"
	"github.com/xmidt-org/wrp-listener/hashTokenFactory"
	secretGetter "github.com/xmidt-org/wrp-listener/secret"
	"github.com/xmidt-org/wrp-listener/webhookClient"

	"go.uber.org/fx"
)

const (
	applicationName = "glaukos"

	DefaultKeyID = "current"
	apiBase      = "api/v1"
)

var (
	GitCommit = "development"
	Version   = "development"
	BuildTime = "development"
)

type SecretConfig struct {
	Header    string
	Delimiter string
}

func main() {
	// setup command line options and configuration from file
	f := pflag.NewFlagSet(applicationName, pflag.ContinueOnError)
	setupFlagSet(f)
	v := viper.New()
	err := setupViper(v, f, applicationName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	app := fx.New(
		xlog.Logger(),
		arrange.Supply(v),
		provideMetrics(),
		basculechecks.ProvideMetrics(),
		basculemetrics.ProvideMetrics(),
		fx.Supply(event.GetLogger),
		event.Provide(),
		fx.Provide(
			ProvideConsts,
			ProvideUnmarshaller,
			xlog.Unmarshal("log"),
			xloghttp.ProvideStandardBuilders,
			xmetricshttp.Unmarshal("prometheus", promhttp.HandlerOpts{}),
			xhttpserver.Unmarshal{Key: "servers.primary", Optional: true}.Annotated(),
			xhttpserver.Unmarshal{Key: "servers.metrics", Optional: true}.Annotated(),
			xhttpserver.Unmarshal{Key: "servers.health", Optional: true}.Annotated(),
			xhealth.Unmarshal("health"),
			provideServerChainFactory,
			arrange.UnmarshalKey("webhook", WebhookConfig{}),
			arrange.UnmarshalKey("secret", SecretConfig{}),
			arrange.UnmarshalKey("queue", queue.QueueConfig{}),
			func(lc fx.Lifecycle, config queue.QueueConfig, parsers parsing.ParsersIn, metrics queue.QueueMetricsIn, logger log.Logger) (*queue.EventQueue, error) {
				q, err := queue.NewEventQueue(config, parsers, metrics, logger)
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						q.Start()
						return nil
					},
					OnStop: func(ctx context.Context) error {
						q.Stop()
						return nil
					},
				})

				return q, err
			},
			func(config WebhookConfig) webhookClient.SecretGetter {
				return secretGetter.NewConstantSecret(config.Request.Config.Secret)
			},
			func(sg webhookClient.SecretGetter) (basculehttp.TokenFactory, error) {
				return hashTokenFactory.New("Sha1", sha1.New, sg)
			},
			func(config WebhookConfig) webhookClient.BasicConfig {
				return webhookClient.BasicConfig{
					Timeout:         config.Timeout,
					RegistrationURL: config.RegistrationURL,
					Request:         config.Request,
				}
			},
			determineTokenAcquirer,
			webhookClient.NewBasicRegisterer,
			func(l fx.Lifecycle, r *webhookClient.BasicRegisterer, c WebhookConfig, logger log.Logger) (*webhookClient.PeriodicRegisterer, error) {
				return webhookClient.NewPeriodicRegisterer(r, c.RegistrationInterval, logger, provider.NewDiscardProvider())
			},
		),
		arrangehttp.Server().Provide(),
		fx.Invoke(
			xhealth.ApplyChecks(
				&health.Config{
					Name:     applicationName,
					Interval: 24 * time.Hour,
					Checker: xhealth.NopCheckable{
						Details: map[string]interface{}{
							"StartTime": time.Now().UTC().Format(time.RFC3339),
						},
					},
				},
			),
			event.ConfigureRoutes,
			BuildMetricsRoutes,
			BuildHealthRoutes,
			func(pr *webhookClient.PeriodicRegisterer) {
				pr.Start()
			},
		),
	)

	if err := app.Err(); err == nil {
		app.Run()
	} else if errors.Is(err, pflag.ErrHelp) {
		return
	} else {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// Provide the constants in the main package for other uber fx components to use.
type ConstOut struct {
	fx.Out
	APIBase      string `name:"api_base"`
	DefaultKeyID string `name:"default_key_id"`
}

func ProvideConsts() ConstOut {
	return ConstOut{
		APIBase:      apiBase,
		DefaultKeyID: DefaultKeyID,
	}
}

// TODO: once we get rid of any packages that need an unmarshaller, remove this.
type UnmarshallerOut struct {
	fx.Out
	Unmarshaller config.Unmarshaller
}

func ProvideUnmarshaller(v *viper.Viper) UnmarshallerOut {
	return UnmarshallerOut{
		Unmarshaller: config.ViperUnmarshaller{Viper: v, Options: []viper.DecoderConfigOption{}},
	}
}
