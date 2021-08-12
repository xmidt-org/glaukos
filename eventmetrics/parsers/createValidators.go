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

	"github.com/xmidt-org/glaukos/eventmetrics/parsers/enums"
	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/history"
	"github.com/xmidt-org/interpreter/validation"
)

// CycleValidationConfig is the config for a cycle validator.
type CycleValidationConfig struct {
	Key                enums.CycleValidationType
	CycleType          string // validate reboot events or last cycle events
	MetadataValidators []string
	EventOrder         []string // the cycle will be sorted in descending order by boot-time, then birthdate
}

// EventValidationConfig is the config for each of the validators.
type EventValidationConfig struct {
	Key                        enums.EventValidationType
	BootTimeValidator          TimeValidationConfig
	BirthdateValidator         TimeValidationConfig
	ValidEventTypes            []string
	MinBootDuration            time.Duration
	BirthdateAlignmentDuration time.Duration
}

func createEventValidator(config EventValidationConfig) (validation.Validator, error) {
	switch config.Key {
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
	case enums.MinBootDurationValidation:
		config = checkTimeValidations(config)
		return validation.BootDurationValidator(config.MinBootDuration), nil
	case enums.BirthdateAlignmentValidation:
		config = checkTimeValidations(config)
		return validation.BirthdateAlignmentValidator(config.BirthdateAlignmentDuration), nil
	case enums.ValidEventTypeValidation:
		return validation.EventTypeValidator(config.ValidEventTypes), nil
	case enums.ConsistentDeviceIDValidation:
		return validation.ConsistentDeviceIDValidator(), nil
	default:
		return nil, errNonExistentKey
	}
}

func createCycleValidator(config CycleValidationConfig) (history.CycleValidator, error) {
	switch config.Key {
	case enums.ConsistentMetadataValidation:
		return history.MetadataValidator(config.MetadataValidators, true), nil
	case enums.UniqueTransactionIDValidation:
		return history.TransactionUUIDValidator(), nil
	case enums.SessionOnlineValidation:
		return history.SessionOnlineValidator(func(events []interpreter.Event, id string) bool {
			return false
		}), nil
	case enums.SessionOfflineValidation:
		return history.SessionOfflineValidator(func(events []interpreter.Event, id string) bool {
			return false
		}), nil
	case enums.EventOrderValidation:
		return history.EventOrderValidator(config.EventOrder), nil
	default:
		return nil, errNonExistentKey
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
