// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package enums

import "strings"

type EventValidationType int

const (
	UnknownEventValidation EventValidationType = iota
	BootTimeValidation
	BirthdateValidation
	MinBootDurationValidation
	BirthdateAlignmentValidation
	ValidEventTypeValidation
	ConsistentDeviceIDValidation
)

const (
	UnknownEventValidationStr       = "unknown"
	BootTimeValidationStr           = "boot-time-validation"
	BirthdateValidationStr          = "birthdate-validation"
	MinBootDurationValidationStr    = "min-boot-duration"
	BirthdateAlignmentValidationStr = "birthdate-alignment"
	ValidEventTypeValidationStr     = "valid-event-type"
	ConsistentDeviceIDValidationStr = "consistent-device-id"
)

func (v *EventValidationType) UnmarshalText(text []byte) error {
	switch strings.ToLower(string(text)) {
	case BootTimeValidationStr:
		*v = BootTimeValidation
	case BirthdateValidationStr:
		*v = BirthdateValidation
	case MinBootDurationValidationStr:
		*v = MinBootDurationValidation
	case BirthdateAlignmentValidationStr:
		*v = BirthdateAlignmentValidation
	case ValidEventTypeValidationStr:
		*v = ValidEventTypeValidation
	case ConsistentDeviceIDValidationStr:
		*v = ConsistentDeviceIDValidation
	default:
		*v = UnknownEventValidation
	}

	return nil
}

func (v EventValidationType) String() string {
	switch v {
	case BootTimeValidation:
		return BootTimeValidationStr
	case BirthdateValidation:
		return BirthdateValidationStr
	case MinBootDurationValidation:
		return MinBootDurationValidationStr
	case BirthdateAlignmentValidation:
		return BirthdateAlignmentValidationStr
	case ValidEventTypeValidation:
		return ValidEventTypeValidationStr
	case ConsistentDeviceIDValidation:
		return ConsistentDeviceIDValidationStr
	}

	return UnknownEventValidationStr
}
