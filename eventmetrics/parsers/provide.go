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
						return false
					}),
					history.SessionOfflineValidator(func(events []interpreter.Event, id string) bool {
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
				name := "reboot_duration_parser"
				logger := parserIn.Logger.With(zap.String("parser", name))
				comparators := history.Comparators([]history.Comparator{
					history.OlderBootTimeComparator(),
				})

				rebootEventFinder := history.LastSessionFinder(validation.DestinationValidator("reboot-pending"))
				parserValidators := createParserValidators(cycleValidator, eventValidator, logger, parserIn.Measures)
				return &RebootDurationParser{
					name:                 name,
					relevantEventsParser: history.LastCycleToCurrentParser(comparators),
					parserValidators:     parserValidators,
					calculators: []DurationCalculator{
						BootDurationCalculator(logger, parserIn.Measures.BootToManageableHistogram),
						TimeBetweenEventsCalculator{
							logger:      logger,
							histogram:   parserIn.Measures.RebootToManageableHistogram,
							eventFinder: rebootEventFinder,
						},
					},
					measures: parserIn.Measures,
					client:   parserIn.CodexClient,
					logger:   logger,
				}
			},
		},
	)
}

func createParserValidators(lastCycleValidator history.CycleValidator, eventValidator validation.Validator, logger *zap.Logger, measures Measures) []ParserValidator {
	rebootEventFinder := history.LastSessionFinder(validation.DestinationValidator("reboot-pending"))
	parserValidators := []ParserValidator{
		&parserValidator{
			cycleParser:     history.LastCycleParser(nil),
			cycleValidator:  lastCycleValidator,
			eventsValidator: eventValidator,
			shouldActivate: func(_ []interpreter.Event, _ interpreter.Event) bool {
				return true
			},
			eventsValidationCallback: func(event interpreter.Event, valid bool, err error) {
				if !valid {
					logEventError(logger, measures.EventErrorTags, err, event)
				}
			},
			cycleValidationCallback: func(valid bool, err error) {
				if !valid {
					logCycleErr(err, measures.BootCycleErrorTags, logger)
				}
			},
		},
		&parserValidator{
			cycleParser:    history.RebootParser(nil),
			cycleValidator: history.EventOrderValidator([]string{"fully-manageable", "operational", "online", "offline", "reboot-pending"}),
			shouldActivate: func(events []interpreter.Event, currentEvent interpreter.Event) bool {
				if _, err := rebootEventFinder.Find(events, currentEvent); err != nil {
					return false
				}
				return true
			},
			cycleValidationCallback: func(valid bool, err error) {
				if !valid {
					logCycleErr(err, measures.RebootCycleErrorTags, logger)
				}
			},
		},
	}

	return parserValidators
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
