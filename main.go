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

package main

import (
	"crypto/sha1" // nolint:gosec
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/go-kit/kit/log"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/arrange/arrangehttp"
	"github.com/xmidt-org/bascule/basculehttp"
	"github.com/xmidt-org/glaukos/eventmetrics"
	"github.com/xmidt-org/httpaux"
	"github.com/xmidt-org/sallust"
	"github.com/xmidt-org/sallust/sallustkit"
	"github.com/xmidt-org/touchstone"
	"github.com/xmidt-org/touchstone/touchhttp"
	"github.com/xmidt-org/wrp-listener/hashTokenFactory"
	secretGetter "github.com/xmidt-org/wrp-listener/secret"
	"github.com/xmidt-org/wrp-listener/webhookClient"

	"go.uber.org/fx"
	"go.uber.org/zap"
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

// nolint:funlen // this is main provide function to hooks up all of the uberfx wiring
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
		arrange.ForViper(v),
		fx.Supply(eventmetrics.GetLogger),
		eventmetrics.Provide(),
		touchhttp.Provide(),
		touchstone.Provide(),
		arrangehttp.Server{
			Key: "servers.health",
			Invoke: arrange.Invoke{
				func(r *mux.Router) {
					r.Handle("/health", httpaux.ConstantHandler{
						StatusCode: http.StatusOK,
					})
				},
			},
		}.Provide(),
		arrangehttp.Server{Key: "servers.metrics"}.Provide(),
		arrangehttp.Server{Key: "servers.primary"}.Provide(),
		webhookClient.Provide(),
		fx.Provide(
			ProvideConsts,
			arrange.UnmarshalKey("prometheus", touchstone.Config{}),
			arrange.UnmarshalKey("log", sallust.Config{}),
			func(config sallust.Config) (*zap.Logger, error) {
				return config.Build()
			},
			func(logger *zap.Logger) log.Logger {
				return sallustkit.Logger{
					Zap: logger,
				}
			},
			arrange.UnmarshalKey("webhook", WebhookConfig{}),
			arrange.UnmarshalKey("secret", SecretConfig{}),
			func(config WebhookConfig) webhookClient.SecretGetter {
				return secretGetter.NewConstantSecret(config.Request.Config.Secret)
			},
			func(sg webhookClient.SecretGetter) (basculehttp.TokenFactory, error) {
				return hashTokenFactory.New("Sha1", sha1.New, sg)
			},
			func(sg webhookClient.SecretGetter, sc SecretConfig, wc WebhookConfig) (alice.Chain, error) {
				if sc.Header != "" && wc.Request.Config.Secret != "" {
					if htf, err := hashTokenFactory.New("Sha1", sha1.New, sg); err != nil {
						return alice.New(), err
					} else {
						authConstructor := basculehttp.NewConstructor(
							basculehttp.WithTokenFactory("Sha1", htf),
							basculehttp.WithHeaderName(sc.Header),
							basculehttp.WithHeaderDelimiter(sc.Delimiter),
						)
						return alice.New(authConstructor), nil
					}
				}
				return alice.New(), nil
			},
			func(config WebhookConfig) webhookClient.BasicConfig {
				return webhookClient.BasicConfig{
					Timeout:         config.Timeout,
					RegistrationURL: config.RegistrationURL,
					Request:         config.Request,
				}
			},
			fx.Annotated{
				Name: "periodic_registration_interval",
				Target: func(config WebhookConfig) time.Duration {
					return config.RegistrationInterval
				},
			},
			determineTokenAcquirer,
		),
		fx.Invoke(
			BuildMetricsRoutes,
			eventmetrics.ConfigureRoutes,
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
