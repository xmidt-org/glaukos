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

	rebootPendingEventType = "reboot-pending"
)

var (
	errNilRebootHistogram = errors.New("reboot_to_manageable histogram cannot be nil")
	errNilBootHistogram   = errors.New("boot_to_manageable histogram cannot be nil")
)

// RebootParserConfig is the config for the reboot duration parser.
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

type RebootParserNameIn struct {
	fx.In
	Name string `name:"reboot_parser_name"`
}

type RebootLoggerIn struct {
	fx.In
	Logger *zap.Logger `name:"reboot_parser_logger"`
}

type RebootParserIn struct {
	fx.In
	Name             string               `name:"reboot_parser_name"`
	Logger           *zap.Logger          `name:"reboot_parser_logger"`
	ParserValidators []ParserValidator    `group:"reboot_parser_validators"`
	Calculators      []DurationCalculator `group:"duration_calculators"`
	Measures         Measures
	CodexClient      *events.CodexClient
}

type CalculatorsIn struct {
	fx.In
	Calculators []DurationCalculator `group:"duration_calculators"`
}

// Provide bundles everything needed for setting up all of the event objects
// for easier wiring into an uber fx application.
func Provide() fx.Option {
	return fx.Options(
		ProvideEventMetrics(),
		events.Provide(),
		provideParsers(),
		provideDurationCalculators(),
		provideParserValidators(),
		fx.Provide(
			arrange.UnmarshalKey("rebootDurationParser", RebootParserConfig{}),
			fx.Annotated{
				Name: "reboot_parser_name",
				Target: func() string {
					return "reboot_duration_parser"
				},
			},
			fx.Annotated{
				Name: "reboot_parser_logger",
				Target: func(parserName RebootParserNameIn, logger *zap.Logger) *zap.Logger {
					return logger.With(zap.String("parser", parserName.Name))
				},
			},
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
			Target: func(parserIn RebootParserIn) queue.Parser {
				comparators := history.Comparators([]history.Comparator{
					history.OlderBootTimeComparator(),
				})

				return &RebootDurationParser{
					name:                 parserIn.Name,
					relevantEventsParser: history.LastCycleToCurrentParser(comparators),
					parserValidators:     parserIn.ParserValidators,
					calculators:          parserIn.Calculators,
					measures:             parserIn.Measures,
					client:               parserIn.CodexClient,
					logger:               parserIn.Logger,
				}
			},
		},
	)
}

func provideDurationCalculators() fx.Option {
	return fx.Provide(
		createBootDurationCallback,
		createRebootToManageableCallback,
		fx.Annotated{
			Group: "duration_calculators",
			Target: func(callback func(interpreter.Event, float64), loggerIn RebootLoggerIn) DurationCalculator {
				return BootDurationCalculator(loggerIn.Logger, callback)
			},
		},
		fx.Annotated{
			Group: "duration_calculators",
			Target: func(callback func(interpreter.Event, interpreter.Event, float64), loggerIn RebootLoggerIn) (DurationCalculator, error) {
				rebootEventFinder := history.LastSessionFinder(validation.DestinationValidator(rebootPendingEventType))
				return NewEventToCurrentCalculator(rebootEventFinder, callback, loggerIn.Logger)
			},
		},
	)
}

func provideParserValidators() fx.Option {
	return fx.Provide(
		fx.Annotated{
			Group: "reboot_parser_validators",
			Target: func(lastCycleValidator history.CycleValidator, eventValidator validation.Validator, loggerIn RebootLoggerIn, m Measures) ParserValidator {
				cycleValidation := cycleValidation{
					validator: lastCycleValidator,
					parser:    history.LastCycleParser(nil),
					callback: func(event interpreter.Event, valid bool, err error) {
						if !valid {
							logCycleErr(event, err, m.BootCycleErrorTags, loggerIn.Logger)
						}
					},
				}

				eventValidation := eventValidation{
					validator: eventValidator,
					callback: func(event interpreter.Event, valid bool, err error) {
						if !valid {
							logEventError(loggerIn.Logger, m.EventErrorTags, err, event)
						}
					},
				}

				return NewParserValidator(
					cycleValidation,
					eventValidation,
					func(_ []interpreter.Event, _ interpreter.Event) bool {
						return true
					},
				)
			},
		},
		fx.Annotated{
			Group: "reboot_parser_validators",
			Target: func(lastCycleValidator history.CycleValidator, eventValidator validation.Validator, loggerIn RebootLoggerIn, m Measures) ParserValidator {
				rebootEventFinder := history.LastSessionFinder(validation.DestinationValidator(rebootPendingEventType))
				cycleValidation := cycleValidation{
					validator: history.EventOrderValidator([]string{"fully-manageable", "operational", "online", "offline", rebootPendingEventType}),
					parser:    history.RebootParser(nil),
					callback: func(event interpreter.Event, valid bool, err error) {
						if !valid {
							logCycleErr(event, err, m.RebootCycleErrorTags, loggerIn.Logger)
						}
					},
				}

				return NewParserValidator(
					cycleValidation,
					eventValidation{},
					func(events []interpreter.Event, currentEvent interpreter.Event) bool {
						if _, err := rebootEventFinder.Find(events, currentEvent); err != nil {
							return false
						}
						return true
					},
				)
			},
		},
	)
}

func createBootDurationCallback(m Measures) (func(interpreter.Event, float64), error) {
	if m.BootToManageableHistogram == nil {
		return nil, errNilBootHistogram
	}

	return func(event interpreter.Event, duration float64) {
		labels := getTimeElapsedHistogramLabels(event)
		m.BootToManageableHistogram.With(labels).Observe(duration)
	}, nil
}

func createRebootToManageableCallback(m Measures) (func(interpreter.Event, interpreter.Event, float64), error) {
	if m.RebootToManageableHistogram == nil {
		return nil, errNilRebootHistogram
	}

	return func(currentEvent interpreter.Event, startingEvent interpreter.Event, duration float64) {
		labels := getTimeElapsedHistogramLabels(currentEvent)
		m.RebootToManageableHistogram.With(labels).Observe(duration)
	}, nil
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
