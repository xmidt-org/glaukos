package validation

import (
	"errors"
	"regexp"
	"time"

	"github.com/xmidt-org/glaukos/message"
)

var (
	ErrInvalidEventType = errors.New("event type doesn't match")

	errNewerBootTime  = errors.New("newer boot-time found")
	errDuplicateEvent = errors.New("duplicate event found")
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

		if valid, err := tv.Valid(time.Unix(bootTime, 0)); !valid {
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

		if valid, err := tv.Valid(time.Unix(0, e.Birthdate)); !valid {
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

// NewestBootTimeValidator returns a ValidatorFunc to check and see if an event's boot-time is
// less than or equal to the latestEvent's boot-time. NewestBootTimeValidator assumes that latestEvent
// has a valid boot-time and does not do any error-checking of latestEvent's boot-time.
func NewestBootTimeValidator(latestEvent message.Event) ValidatorFunc {
	latestBootTime, _ := latestEvent.BootTime()
	return func(e message.Event) (bool, error) {
		// event is latestEvent, no need to compare boot-times
		if e.TransactionUUID == latestEvent.TransactionUUID {
			return true, nil
		}

		bootTime, err := e.BootTime()
		if err != nil || bootTime <= 0 {
			return true, nil
		}

		// if this event has a boot-time more recent than the latest one, return an error
		if bootTime > latestBootTime {
			return false, InvalidEventErr{OriginalErr: InvalidBootTimeErr{OriginalErr: errNewerBootTime}}
		}

		return true, nil
	}
}

// UniqueEventValidator returns a ValidatorFunc to check and see if compareEvent is unique, meaning that
// it does not share the same destination type and boot-time as another event.
// UniqueEventValidator assumes that compareEvent has a valid boot-time
// and does not do any error-checking of compareEvent's boot-time.
func UniqueEventValidator(compareEvent message.Event, eventType *regexp.Regexp) ValidatorFunc {
	destValidator := DestinationValidator(eventType)
	latestBootTime, _ := compareEvent.BootTime()
	return func(e message.Event) (bool, error) {
		// event is latestEvent, no need to compare boot-times
		if e.TransactionUUID == compareEvent.TransactionUUID {
			return true, nil
		}

		// see if event is the type we are looking for
		if destMatch, _ := destValidator.Valid(e); destMatch {
			bootTime, err := e.BootTime()
			if err != nil || bootTime <= 0 {
				return true, nil
			}

			// If the boot-time is the same as the latestBootTime, and the birthdate is older or equal,
			// this means that compareEvent is a duplicate.
			if bootTime == latestBootTime && e.Birthdate <= compareEvent.Birthdate {
				return false, InvalidEventErr{OriginalErr: errDuplicateEvent}
			}
		}

		return true, nil
	}
}
