package validation

import (
	"errors"
	"regexp"
	"time"
)

// RuleConfig is the config struct for a rule
type RuleConfig struct {
	Regex          string
	CalculateUsing string
	ValidFrom      time.Duration
}

var (
	ErrInvalidEventType = errors.New("event type doesn't match")
	ErrInvalidBootTime  = errors.New("invalid boot-time")
	ErrInvalidBirthdate = errors.New("invalid birthdate")
	ErrTimeParse        = errors.New("parsing error")

	errInvalidRegex = errors.New("invalid regex")
)

// NewEventValidators creates a set of new validators from a RuleConfig
func NewEventValidators(config RuleConfig, validTo time.Duration, currentTime func() time.Time) (Validators, error) {
	regex, err := regexp.Compile(config.Regex)
	if err != nil {
		return nil, errInvalidRegex
	}

	validators := make(Validators, 0, 3)
	tv := TimeValidator{ValidFrom: config.ValidFrom, ValidTo: validTo, Current: currentTime}

	if ParseTimeLocation(config.CalculateUsing) == Birthdate {
		validators = append(validators, BirthdateValidator(tv))
	}
	validators = append(validators, BootTimeValidator(tv), DestinationValidator(regex))

	return validators, nil
}
