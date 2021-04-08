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

package parsers

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/glaukos/eventmetrics/queue"
	"github.com/xmidt-org/glaukos/events"
	"github.com/xmidt-org/touchstone"
	"go.uber.org/fx"
)

var (
	errInvalidName = errors.New("invalid parser name")
)

type TimeElapsedParsersConfig struct {
	DefaultValidFrom time.Duration
	Parsers          []TimeElapsedConfig
}

type TimeElapsedParsersIn struct {
	fx.In
	Config      TimeElapsedParsersConfig
	Logger      log.Logger
	Measures    Measures
	CodexClient *events.CodexClient
	Factory     *touchstone.Factory
}

// Provide bundles everything needed for setting up all of the event objects
// for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideEventMetrics(),
		events.Provide(),
		provideParsers(),
		fx.Provide(
			arrange.UnmarshalKey("timeElapsedParsers", TimeElapsedParsersConfig{}),
			TimeElapsedParsers,
		),
	)
}

func provideParsers() fx.Option {
	return fx.Provide(
		fx.Annotated{
			Group: "parsers",
			Target: func(measures Measures, logger log.Logger) queue.Parser {
				return &MetadataParser{
					measures: measures,
					name:     "metadata",
					logger:   ParserLogger(logger, "metadata"),
				}
			},
		},
		fx.Annotated{
			Group:  "parsers,flatten",
			Target: TimeElapsedParsers,
		},
	)
}

// TimeElapsedParsers creates a list of TimeElapsedParsers from the config.
func TimeElapsedParsers(parsers TimeElapsedParsersIn) ([]queue.Parser, error) {
	if valid, err := validNames(parsers.Config.Parsers); !valid {
		return nil, err
	}

	if parsers.Config.DefaultValidFrom == 0 {
		parsers.Config.DefaultValidFrom = -1 * time.Hour
	}

	parsersList := make([]queue.Parser, 0, len(parsers.Config.Parsers))
	for _, parserConfig := range parsers.Config.Parsers {
		parserConfig = fixConfig(parserConfig, parsers.Config.DefaultValidFrom)
		o := prometheus.HistogramOpts{
			Name:    parserConfig.Name,
			Help:    fmt.Sprintf("tracks %s durations in s", parserConfig.Name),
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		}

		if added, err := parsers.Measures.addTimeElapsedHistogram(parsers.Factory, o, firmwareLabel, hardwareLabel, rebootReasonLabel); !added {
			return nil, err
		}

		logger := ParserLogger(parsers.Logger, parserConfig.Name)
		parser, err := NewTimeElapsedParser(parserConfig, parsers.CodexClient, logger, parsers.Measures, time.Now)
		if err != nil {
			return nil, err
		}

		parsersList = append(parsersList, parser)
	}

	return parsersList, nil
}

// validNames checks that all of the time elapsed parsers have unique names.
func validNames(parsers []TimeElapsedConfig) (bool, error) {
	names := make(map[string]bool)
	for _, parser := range parsers {
		if len(parser.Name) == 0 {
			return false, fmt.Errorf("%w: name cannot be blank", errInvalidName)
		}
		if names[parser.Name] {
			return false, fmt.Errorf("%w: %s is already used by another parser", errInvalidName, parser.Name)
		}
		names[parser.Name] = true
	}

	return true, nil
}

// ParserLogger pulls the logger from the context and adds the parser name to it.
func ParserLogger(logger log.Logger, parserName string) log.Logger {
	return log.With(logger, "parser", parserName)
}
