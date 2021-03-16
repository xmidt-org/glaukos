package validation

import (
	"errors"
	"regexp"
	"time"

	"github.com/xmidt-org/glaukos/message"
)

var (
	ErrInvalidEventType = errors.New("event type doesn't match")
	ErrInvalidBootTime  = errors.New("invalid boot-time")
	ErrInvalidBirthdate = errors.New("invalid birthdate")
	ErrTimeParse        = errors.New("parsing error")

	errInvalidRegex = errors.New("invalid regex")
)

// Validator validates an event, returning false and an error if the event is not valid
// and true if the event is valid
type Validator interface {
	Valid(message.Event) (bool, error)
}

// ValidatorFunc is a function that checks if an Event is valid
type ValidatorFunc func(message.Event) (bool, error)

// Valid runs the ValidatorFunc, making a ValidatorFunc a Validator
func (vf ValidatorFunc) Valid(e message.Event) (bool, error) {
	return vf(e)
}

// Validators are a list of objects that implement the Validator interface
type Validators []Validator

// Valid runs through a list of Validators and checks that the Event
// is valid against each validator. Returns false and an error at the first
// validator that deems the Event invalid
func (v Validators) Valid(e message.Event) (bool, error) {
	for _, r := range v {
		if valid, err := r.Valid(e); !valid {
			return false, err
		}
	}
	return true, nil
}

// ValidatorConfig is the config struct holding information to create validators for an event.
type ValidatorConfig struct {
	Regex          string
	CalculateUsing string
	ValidFrom      time.Duration
}

// NewEventValidators creates a set of validators from a ValidatorConfig
func NewEventValidators(config ValidatorConfig, validTo time.Duration, currentTime func() time.Time) (Validators, error) {
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

// BootTimeValidator returns a ValidatorFunc that checks if an
// Event's boot-time is valid, meaning parsable, greater than 0, and within the
// bounds deemed valid by the TimeValidation parameter.
func BootTimeValidator(tv TimeValidation) ValidatorFunc {
	return func(e message.Event) (bool, error) {
		bootTime, err := e.BootTime()
		if err != nil || bootTime <= 0 {
			return false, InvalidEventErr{
				OriginalErr: InvalidBootTimeErr{
					OriginalErr: err,
				},
			}
		}

		if valid, err := tv.ValidTime(time.Unix(bootTime, 0)); !valid {
			return false, InvalidEventErr{
				OriginalErr: InvalidBootTimeErr{
					OriginalErr: err,
				},
			}
		}

		return true, nil
	}
}

// BirthdateValidator returns a ValidatorFunc that checks if an
// Event's birthdate is valid, meaning greater than 0 and within the
// bounds deemed valid by the TimeValidation parameter.
func BirthdateValidator(tv TimeValidation) ValidatorFunc {
	return func(e message.Event) (bool, error) {
		birthdate := e.Birthdate
		if birthdate <= 0 {
			return false, InvalidEventErr{
				OriginalErr: InvalidBirthdateErr{},
			}
		}

		if valid, err := tv.ValidTime(time.Unix(0, e.Birthdate)); !valid {
			return false, InvalidEventErr{
				OriginalErr: InvalidBirthdateErr{
					OriginalErr: err,
				},
			}
		}

		return true, nil
	}
}

// DestinationValidator takes in a regex and returns a ValidatorFunc that checks if an
// Event's destination is valid against this regex.
func DestinationValidator(regex *regexp.Regexp) ValidatorFunc {
	return func(e message.Event) (bool, error) {
		if !regex.MatchString(e.Destination) {
			return false, InvalidEventErr{OriginalErr: ErrInvalidEventType}
		}
		return true, nil
	}
}
