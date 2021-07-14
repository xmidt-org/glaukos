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
	errWrongEventValidatorKey = errors.New("not an event validator key")
	errWrongCycleValidatorKey = errors.New("not a cycle validator key")
	errNonExistentKey         = errors.New("key does not exist")
)

type RebootParserConfig struct {
	EventValidators []EventValidationConfig
	CycleValidators []CycleValidationConfig
}

type CycleValidationConfig struct {
	Key                string
	CycleType          string
	MetadataValidators []string
}

// ValidationConfig is the config for each of the validators
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
			func(config RebootParserConfig) (history.CycleValidator, error) {
				return createCycleValidators(config.CycleValidators)
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
			Target: func(cycleValidator history.CycleValidator, eventValidator validation.Validator, parserIn RebootParserIn) queue.Parser {
				name := "reboot_duration_parser"
				logger := parserIn.Logger.With(zap.String("parser", name))
				comparators := history.Comparators([]history.Comparator{
					history.OlderBootTimeComparator(),
				})

				rebootEventFinder := history.LastSessionFinder(validation.DestinationValidator(rebootPendingEventType))
				parserValidators := createParserValidators(cycleValidator, eventValidator, logger, parserIn.Measures)
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

func createCycleValidator(config CycleValidationConfig) (history.CycleValidator, error) {
	validationType := ParseValidationType(config.Key)
	if validationType == unknown {
		return nil, errNonExistentKey
	}

	switch validationType {
	case consistentMetadata:
		return history.MetadataValidator(config.MetadataValidators, true), nil
	case uniqueTransactionID:
		return history.TransactionUUIDValidator(), nil
	case sessionOnline:
		return history.SessionOnlineValidator(func(events []interpreter.Event, id string) bool {
			if len(events) > 0 {
				return id == events[0].SessionID
			}
			return false
		}), nil
	case sessionOffline:
		return history.SessionOfflineValidator(func(events []interpreter.Event, id string) bool {
			if len(events) > 0 {
				return id == events[len(events)-1].SessionID
			}
			return false
		}), nil
	default:
		return nil, errWrongCycleValidatorKey
	}
}
func createBootDurationCallback(m Measures) func(interpreter.Event, float64) {
	return func(event interpreter.Event, duration float64) {
		labels := getTimeElapsedHistogramLabels(event)
		m.BootToManageableHistogram.With(labels).Observe(duration)
	}
}

func createRebootToManageableCallback(m Measures) func(interpreter.Event, interpreter.Event, float64) {
	return func(currentEvent interpreter.Event, startingEvent interpreter.Event, duration float64) {
		labels := getTimeElapsedHistogramLabels(currentEvent)
		m.RebootToManageableHistogram.With(labels).Observe(duration)
	}
}

func createParserValidators(lastCycleValidator history.CycleValidator, eventValidator validation.Validator, logger *zap.Logger, measures Measures) []ParserValidator {
	rebootEventFinder := history.LastSessionFinder(validation.DestinationValidator(rebootPendingEventType))
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
			cycleValidator: history.EventOrderValidator([]string{"fully-manageable", "operational", "online", "offline", rebootPendingEventType}),
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

func createEventValidator(config EventValidationConfig) (validation.Validator, error) {
	validationType := ParseValidationType(config.Key)
	if validationType == unknown {
		return nil, errNonExistentKey
	}

	switch validationType {
	case bootTimeValidation:
		config = checkTimeValidations(config)
		bootTimeValidator := validation.TimeValidator{
			Current:      time.Now,
			ValidFrom:    config.BootTimeValidator.ValidFrom,
			ValidTo:      config.BootTimeValidator.ValidTo,
			MinValidYear: config.BootTimeValidator.MinValidYear,
		}
		return validation.BootTimeValidator(bootTimeValidator), nil
	case birthdateValidation:
		config = checkTimeValidations(config)
		birthdateValidator := validation.TimeValidator{
			Current:      time.Now,
			ValidFrom:    config.BootTimeValidator.ValidFrom,
			ValidTo:      config.BootTimeValidator.ValidTo,
			MinValidYear: config.BootTimeValidator.MinValidYear,
		}
		return validation.BirthdateValidator(birthdateValidator), nil
	case minBootDuration:
		config = checkTimeValidations(config)
		return validation.BootDurationValidator(config.MinBootDuration), nil
	case birthdateAlignment:
		config = checkTimeValidations(config)
		return validation.BirthdateAlignmentValidator(config.BirthdateAlignmentDuration), nil
	case validEventType:
		return validation.EventTypeValidator(config.ValidEventTypes), nil
	case consistentDeviceID:
		return validation.ConsistentDeviceIDValidator(), nil
	default:
		return nil, errWrongEventValidatorKey
	}
}

func createCycleValidators(configs []CycleValidationConfig) (history.CycleValidator, error) {
	// TODO: add for different cycle checks
	var validators history.CycleValidators
	for _, config := range configs {
		validator, err := createCycleValidator(config)
		if err != nil {
			return nil, err
		}
		validators = append(validators, validator)
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
