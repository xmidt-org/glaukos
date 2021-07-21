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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/interpreter/validation"
	"github.com/xmidt-org/touchstone"

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
	errWrongEventValidatorKey = errors.New("not an event validator key")
	errWrongCycleValidatorKey = errors.New("not a cycle validator key")
	errNonExistentKey         = errors.New("key does not exist")
	errBlankHistogramName     = errors.New("name cannot be blank")
	errNilHistogram           = errors.New("histogram missing")
	errNilBootHistogram       = errors.New("boot_to_manageable histogram cannot be nil")
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

// CycleValidationConfig is the config for a cycle validator.
type CycleValidationConfig struct {
	Key                string
	CycleType          string // validate reboot events or last cycle events
	MetadataValidators []string
	EventOrder         []string // the cycle will be sorted in descending order by boot-time, then birthdate
}

// EventValidationConfig is the config for each of the validators.
type EventValidationConfig struct {
	Key                        string
	BootTimeValidator          TimeValidationConfig
	BirthdateValidator         TimeValidationConfig
	ValidEventTypes            []string
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

type ValidatorsIn struct {
	fx.In
	EventValidator       validation.Validator
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
			func(config RebootParserConfig) (validation.Validator, error) {
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
		// createRebootToManageableCallback,
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

func createDurationCalculators(f *touchstone.Factory, configs []TimeElapsedConfig, m Measures, loggerIn RebootLoggerIn) ([]DurationCalculator, error) {
	calculators := make([]DurationCalculator, len(configs))
	for i, config := range configs {
		if len(config.Name) == 0 {
			return nil, errBlankHistogramName
		}

		options := prometheus.HistogramOpts{
			Name:    config.Name,
			Help:    fmt.Sprintf("time elapsed between a %s event and fully-manageable event in s", config.EventType),
			Buckets: []float64{60, 120, 180, 240, 300, 360, 420, 480, 540, 600, 900, 1200, 1500, 1800, 3600, 7200, 14400, 21600},
		}

		if err := m.addTimeElapsedHistogram(f, options, firmwareLabel, hardwareLabel, rebootReasonLabel); err != nil {
			return nil, err
		}

		sessionType := enums.ParseSessionType(config.SessionType)
		var finder Finder
		if sessionType == enums.Previous {
			finder = history.LastSessionFinder(validation.DestinationValidator(config.EventType))
		} else {
			finder = history.CurrentSessionFinder(validation.DestinationValidator(config.EventType))
		}

		callback, err := createTimeElapsedCallback(m, config.Name)
		if err != nil {
			return nil, err
		}

		calculator, err := NewEventToCurrentCalculator(finder, callback, loggerIn.Logger)
		if err != nil {
			return nil, err
		}

		calculators[i] = calculator
	}

	return calculators, nil
}

func createBootDurationCallback(m Measures) (func(interpreter.Event, float64), error) {
	if m.BootToManageableHistogram == nil {
		return nil, errNilBootHistogram
	}

	return func(event interpreter.Event, duration float64) {
		AddDuration(m.BootToManageableHistogram, duration, event)
	}, nil
}

func createTimeElapsedCallback(m Measures, name string) (func(interpreter.Event, interpreter.Event, float64), error) {
	if m.TimeElapsedHistograms == nil {
		return nil, errNilHistogram
	}

	histogram, found := m.TimeElapsedHistograms[name]

	if !found {
		return nil, errNilHistogram
	}

	return func(currentEvent interpreter.Event, startingEvent interpreter.Event, duration float64) {
		AddDuration(histogram, duration, currentEvent)
	}, nil
}

func createEventValidator(config EventValidationConfig) (validation.Validator, error) {
	validationType := enums.ParseValidationType(config.Key)
	if validationType == enums.UnknownValidation {
		return nil, errNonExistentKey
	}

	switch validationType {
	case enums.BootTimeValidation:
		config = checkTimeValidations(config)
		bootTimeValidator := validation.TimeValidator{
			Current:      time.Now,
			ValidFrom:    config.BootTimeValidator.ValidFrom,
			ValidTo:      config.BootTimeValidator.ValidTo,
			MinValidYear: config.BootTimeValidator.MinValidYear,
		}
		return validation.BootTimeValidator(bootTimeValidator), nil
	case enums.BirthdateValidation:
		config = checkTimeValidations(config)
		birthdateValidator := validation.TimeValidator{
			Current:      time.Now,
			ValidFrom:    config.BootTimeValidator.ValidFrom,
			ValidTo:      config.BootTimeValidator.ValidTo,
			MinValidYear: config.BootTimeValidator.MinValidYear,
		}
		return validation.BirthdateValidator(birthdateValidator), nil
	case enums.MinBootDuration:
		config = checkTimeValidations(config)
		return validation.BootDurationValidator(config.MinBootDuration), nil
	case enums.BirthdateAlignment:
		config = checkTimeValidations(config)
		return validation.BirthdateAlignmentValidator(config.BirthdateAlignmentDuration), nil
	case enums.ValidEventType:
		return validation.EventTypeValidator(config.ValidEventTypes), nil
	case enums.ConsistentDeviceID:
		return validation.ConsistentDeviceIDValidator(), nil
	default:
		return nil, errWrongEventValidatorKey
	}
}

func createCycleValidator(config CycleValidationConfig) (history.CycleValidator, error) {
	validationType := enums.ParseValidationType(config.Key)
	if validationType == enums.UnknownValidation {
		return nil, errNonExistentKey
	}

	switch validationType {
	case enums.ConsistentMetadata:
		return history.MetadataValidator(config.MetadataValidators, true), nil
	case enums.UniqueTransactionID:
		return history.TransactionUUIDValidator(), nil
	case enums.SessionOnline:
		return history.SessionOnlineValidator(func(events []interpreter.Event, id string) bool {
			return false
		}), nil
	case enums.SessionOffline:
		return history.SessionOfflineValidator(func(events []interpreter.Event, id string) bool {
			return false
		}), nil
	case enums.EventOrder:
		return history.EventOrderValidator(config.EventOrder), nil
	default:
		return nil, errWrongCycleValidatorKey
	}
}

func createCycleValidators(configs []CycleValidationConfig, cycleType enums.CycleType) (history.CycleValidator, error) {
	var validators history.CycleValidators
	for _, config := range configs {
		if enums.ParseCycleType(config.CycleType) == cycleType {
			validator, err := createCycleValidator(config)
			if err != nil {
				return nil, err
			}
			validators = append(validators, validator)
		}
	}

	return validators, nil
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
