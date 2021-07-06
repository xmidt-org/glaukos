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
	"time"

	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/interpreter/validation"

	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/glaukos/eventmetrics/queue"
	"github.com/xmidt-org/glaukos/events"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	defaultValidFrom                  = -8766 * time.Hour // 1 year
	defaultValidTo                    = time.Hour
	defaultMinBootDuration            = 10 * time.Second
	defaultBirthdateAlignmentDuration = 60 * time.Second
)

// RebootParserConfig is the config for the reboot duration parser
type RebootParserConfig struct {
	ValidEventTypes            []string
	MetadataValidators         []string
	BootTimeValidator          TimeValidationConfig
	BirthdateValidator         TimeValidationConfig
	MinBootDuration            time.Duration
	BirthdateAlignmentDuration time.Duration
}

// TimeValidationConfig is the config used for time validation.
type TimeValidationConfig struct {
	ValidFrom    time.Duration
	ValidTo      time.Duration
	MinValidYear int
}

type RebootParserIn struct {
	fx.In
	Logger      *zap.Logger
	Measures    Measures
	CodexClient *events.CodexClient
}

// Provide bundles everything needed for setting up all of the event objects
// for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideEventMetrics(),
		events.Provide(),
		provideParsers(),
		fx.Provide(
			arrange.UnmarshalKey("rebootDurationParser", RebootParserConfig{}),
			createEventValidator,
			func(config RebootParserConfig) history.CycleValidator {
				validators := []history.CycleValidator{
					history.TransactionUUIDValidator(),
					history.MetadataValidator(config.MetadataValidators, true),
					history.SessionOnlineValidator(func(events []interpreter.Event, id string) bool {
						if len(events) > 0 {
							return id == events[0].SessionID
						}
						return false
					}),
					history.SessionOfflineValidator(func(events []interpreter.Event, id string) bool {
						if len(events) > 0 {
							return id == events[len(events)-1].SessionID
						}
						return false
					}),
				}

				return history.CycleValidators(validators)
			},
		),
	)
}

func provideParsers() fx.Option {
	return fx.Provide(
		fx.Annotated{
			Group: "parsers",
			Target: func(measures Measures, logger *zap.Logger) queue.Parser {
				return &MetadataParser{
					measures: measures,
					name:     "metadata",
					logger:   logger.With(zap.String("parser", "metadata")),
				}
			},
		},
		fx.Annotated{
			Group: "parsers",
			Target: func(cycleValidator history.CycleValidator, eventValidator validation.Validator, parserIn RebootParserIn) queue.Parser {
				comparators := history.Comparators([]history.Comparator{
					history.OlderBootTimeComparator(),
				})

				return &RebootDurationParser{
					name:             "reboot_duration_parser",
					finder:           history.LastSessionFinder(validation.DestinationValidator("reboot-pending")),
					cycleValidator:   cycleValidator,
					cycleParser:      history.LastCycleToCurrentParser(comparators),
					validationParser: history.LastCycleParser(comparators),
					eventValidator:   eventValidator,
					measures:         parserIn.Measures,
					client:           parserIn.CodexClient,
					logger:           parserIn.Logger,
				}
			},
		},
	)
}

func createEventValidator(config RebootParserConfig) validation.Validator {
	config = checkTimeValidations(config)

	bootTimeValidator := validation.TimeValidator{
		Current:      time.Now,
		ValidFrom:    config.BootTimeValidator.ValidFrom,
		ValidTo:      config.BootTimeValidator.ValidTo,
		MinValidYear: config.BootTimeValidator.MinValidYear,
	}
	birthdateValidator := validation.TimeValidator{
		Current:      time.Now,
		ValidFrom:    config.BirthdateValidator.ValidFrom,
		ValidTo:      config.BirthdateValidator.ValidTo,
		MinValidYear: config.BirthdateValidator.MinValidYear,
	}

	validators := []validation.Validator{
		validation.EventTypeValidator(config.ValidEventTypes),
		validation.ConsistentDeviceIDValidator(),
		validation.BootDurationValidator(config.MinBootDuration),
		validation.BirthdateAlignmentValidator(config.BirthdateAlignmentDuration),
		validation.BootTimeValidator(bootTimeValidator),
		validation.BirthdateValidator(birthdateValidator),
	}

	return validation.Validators(validators)
}

func checkTimeValidations(config RebootParserConfig) RebootParserConfig {
	if config.BootTimeValidator.ValidFrom == 0 {
		config.BootTimeValidator.ValidFrom = defaultValidFrom
	}

	if config.BirthdateValidator.ValidFrom == 0 {
		config.BirthdateValidator.ValidFrom = defaultValidFrom
	}

	if config.BootTimeValidator.ValidTo == 0 {
		config.BootTimeValidator.ValidTo = defaultValidTo
	}

	if config.BirthdateValidator.ValidTo == 0 {
		config.BirthdateValidator.ValidTo = defaultValidTo
	}

	if config.MinBootDuration == 0 {
		config.MinBootDuration = defaultMinBootDuration
	}

	if config.BirthdateAlignmentDuration == 0 {
		config.BirthdateAlignmentDuration = defaultBirthdateAlignmentDuration
	}

	return config
}
