// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"errors"
	"time"

	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/interpreter/validation"

	"github.com/xmidt-org/arrange"
	"github.com/xmidt-org/glaukos/eventmetrics/parsers/enums"
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
	errBlankHistogramName = errors.New("name cannot be blank")
	errNilHistogram       = errors.New("histogram missing")
	errNilBootHistogram   = errors.New("boot_to_manageable histogram cannot be nil")
)

// RebootParserConfig contains the information for which validators should be created.
type RebootParserConfig struct {
	EventValidators         []EventValidationConfig
	CycleValidators         []CycleValidationConfig
	TimeElapsedCalculations []TimeElapsedConfig
}

// TimeElapsedConfig contains information for calculating the time between a fully-manageable event and another event.
type TimeElapsedConfig struct {
	Name        string
	SessionType string
	EventType   string
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

type ValidatorsIn struct {
	fx.In
	EventValidator       validation.Validator   `name:"event_validator"`
	LastCycleValidator   history.CycleValidator `name:"last_cycle_validator"`
	RebootCycleValidator history.CycleValidator `name:"reboot_cycle_validator"`
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
			arrange.UnmarshalKey("rebootDurationParser.timeElapsedCalculations", []TimeElapsedConfig{}),
			fx.Annotated{
				Name: "reboot_parser_name",
				Target: func() string {
					return "reboot_duration_parser"
				},
			},
			fx.Annotated{
				Name: "event_validator",
				Target: func(config RebootParserConfig) (validation.Validator, error) {
					var validators validation.Validators
					for _, config := range config.EventValidators {
						validator, err := createEventValidator(config)
						if err != nil {
							return nil, err
						}
						validators = append(validators, validator)
					}
					return validators, nil
				},
			},
			fx.Annotated{
				Name: "last_cycle_validator",
				Target: func(config RebootParserConfig) (history.CycleValidator, error) {
					return createCycleValidators(config.CycleValidators, enums.BootTime)
				},
			},
			fx.Annotated{
				Name: "reboot_cycle_validator",
				Target: func(config RebootParserConfig) (history.CycleValidator, error) {
					return createCycleValidators(config.CycleValidators, enums.Reboot)
				},
			},
			fx.Annotated{
				Name: "reboot_parser_logger",
				Target: func(parserName RebootParserNameIn, logger *zap.Logger) *zap.Logger {
					return logger.With(zap.String("parser", parserName.Name))
				},
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
		fx.Annotated{
			Group: "duration_calculators",
			Target: func(callback func(interpreter.Event, float64), loggerIn RebootLoggerIn) DurationCalculator {
				return BootDurationCalculator(loggerIn.Logger, callback)
			},
		},
		fx.Annotated{
			Group:  "duration_calculators,flatten",
			Target: createDurationCalculators,
		},
	)
}

func provideParserValidators() fx.Option {
	return fx.Provide(
		fx.Annotated{
			Group: "reboot_parser_validators",
			Target: func(validatorsIn ValidatorsIn, loggerIn RebootLoggerIn, m Measures) ParserValidator {
				cycleValidation := cycleValidation{
					validator: validatorsIn.LastCycleValidator,
					parser:    history.LastCycleParser(nil),
					callback: func(event interpreter.Event, valid bool, err error) {
						if !valid {
							logCycleErr(event, err, m.BootCycleErrorTags, loggerIn.Logger)
						}
					},
				}

				eventValidation := eventValidation{
					validator: validatorsIn.EventValidator,
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
			Target: func(validatorsIn ValidatorsIn, loggerIn RebootLoggerIn, m Measures) ParserValidator {
				rebootEventFinder := history.LastSessionFinder(validation.DestinationValidator(rebootPendingEventType))
				cycleValidation := cycleValidation{
					validator: validatorsIn.RebootCycleValidator,
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

func checkTimeValidations(config EventValidationConfig) EventValidationConfig {
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
