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
)

// RebootParserConfig contains the information for which validators should be created.
type RebootParserConfig struct {
	EventValidators []EventValidationConfig
	CycleValidators []CycleValidationConfig
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

type RebootParserIn struct {
	fx.In
	BootCycleValidator   history.CycleValidator `name:"boot_cycle_validator"`
	RebootCycleValidator history.CycleValidator `name:"reboot_cycle_validator"`
	EventValidator       validation.Validator
	Logger               *zap.Logger
	Measures             Measures
	CodexClient          *events.CodexClient
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
			fx.Annotated{
				Name: "boot_cycle_validator",
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
				name := "reboot_duration_parser"
				logger := parserIn.Logger.With(zap.String("parser", name))
				comparators := history.Comparators([]history.Comparator{
					history.OlderBootTimeComparator(),
				})

				rebootEventFinder := history.LastSessionFinder(validation.DestinationValidator(rebootPendingEventType))
				parserValidators := createParserValidators(parserIn, logger)
				return &RebootDurationParser{
					name:                 name,
					relevantEventsParser: history.LastCycleToCurrentParser(comparators),
					parserValidators:     parserValidators,
					calculators: []DurationCalculator{
						BootDurationCalculator(logger, createBootDurationCallback(parserIn.Measures)),
						&EventToCurrentCalculator{
							logger:          logger,
							successCallback: createRebootToManageableCallback(parserIn.Measures),
							eventFinder:     rebootEventFinder,
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

func createParserValidators(parserIn RebootParserIn, logger *zap.Logger) []ParserValidator {
	rebootEventFinder := history.LastSessionFinder(validation.DestinationValidator(rebootPendingEventType))
	parserValidators := []ParserValidator{
		&parserValidator{
			cycleParser:     history.LastCycleParser(nil),
			cycleValidator:  parserIn.BootCycleValidator,
			eventsValidator: parserIn.EventValidator,
			shouldActivate: func(_ []interpreter.Event, _ interpreter.Event) bool {
				return true
			},
			eventsValidationCallback: func(event interpreter.Event, valid bool, err error) {
				if !valid {
					logEventError(logger, parserIn.Measures.EventErrorTags, err, event)
				}
			},
			cycleValidationCallback: func(valid bool, err error) {
				if !valid {
					logCycleErr(err, parserIn.Measures.BootCycleErrorTags, logger)
				}
			},
		},
		&parserValidator{
			cycleParser:    history.RebootParser(nil),
			cycleValidator: parserIn.RebootCycleValidator,
			shouldActivate: func(events []interpreter.Event, currentEvent interpreter.Event) bool {
				if _, err := rebootEventFinder.Find(events, currentEvent); err != nil {
					return false
				}
				return true
			},
			cycleValidationCallback: func(valid bool, err error) {
				if !valid {
					logCycleErr(err, parserIn.Measures.RebootCycleErrorTags, logger)
				}
			},
		},
	}

	return parserValidators
}

func createBootDurationCallback(m Measures) func(interpreter.Event, float64) {
	return func(event interpreter.Event, duration float64) {
		if m.BootToManageableHistogram != nil {
			labels := getTimeElapsedHistogramLabels(event)
			m.BootToManageableHistogram.With(labels).Observe(duration)
		}

	}
}

func createRebootToManageableCallback(m Measures) func(currentEvent interpreter.Event, startingEvent interpreter.Event, duration float64) {
	return func(currentEvent interpreter.Event, startingEvent interpreter.Event, duration float64) {
		if m.RebootToManageableHistogram != nil {
			labels := getTimeElapsedHistogramLabels(currentEvent)
			m.RebootToManageableHistogram.With(labels).Observe(duration)
		}
	}
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
