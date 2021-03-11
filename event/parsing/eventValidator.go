package parsing

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/queue"
)

// EventValidation is an interface for checking if wrp messages and events pass certain validations
type EventValidation interface {
	IsEventValid(client.Event) (bool, error)
	IsWRPValid(queue.WrpWithTime) (bool, error)
	GetEventCompareTime(client.Event) (time.Time, error)
	GetWRPCompareTime(queue.WrpWithTime) (time.Time, error)
	ValidateType(string) bool
}

// EventRule is the config struct for event validation
type EventRule struct {
	Regex          string
	CalculateUsing string
	ValidFrom      time.Duration
}

// eventValidator implements a metricparser.EventValidator interface.
// Keeps a set of information/rules to see if an event or wrp fits these rules.
type eventValidator struct {
	regex          *regexp.Regexp
	calculateUsing TimeLocation
	timeValidation TimeValidation
}

var (
	ErrInvalidEventType = errors.New("event type doesn't match")
	ErrInvalidRegex     = errors.New("invalid regex")
	ErrInvalidBootTime  = errors.New("invalid boot-time")
	ErrInvalidBirthdate = errors.New("invalid birthdate")
	ErrTimeParse        = errors.New("parsing error")
)

// NewEventValidation creates a new EventValidator from an EventRule
func NewEventValidation(rule EventRule, validTo time.Duration, currentTime func() time.Time) (EventValidation, error) {
	regex, err := regexp.Compile(rule.Regex)
	if err != nil {
		return eventValidator{}, fmt.Errorf("%w: %v", ErrInvalidRegex, err)
	}

	tv := TimeValidator{Current: currentTime, ValidFrom: rule.ValidFrom, ValidTo: validTo}

	timeLocation := ParseTimeLocation(rule.CalculateUsing)

	return eventValidator{
		regex:          regex,
		calculateUsing: timeLocation,
		timeValidation: tv,
	}, nil
}

// IsEventValid checks if an event is valid based on the rules kept by the EventValidator.
func (e eventValidator) IsEventValid(event client.Event) (bool, error) {
	bootTime, err := GetEventBootTime(event)
	if err != nil || bootTime <= 0 {
		return false, ErrInvalidBootTime
	}

	// see if event found matches expected event type
	if !e.ValidateType(event.Dest) {
		return false, fmt.Errorf("%w. Desired type: %s", ErrInvalidEventType, e.regex.String())
	}

	compareTime, err := e.GetEventCompareTime(event)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrTimeParse, err)
	}

	if valid, err := e.timeValidation.IsTimeValid(compareTime); !valid {
		if e.calculateUsing == Birthdate {
			return false, fmt.Errorf("%w: %v", ErrInvalidBirthdate, err)
		}
		return false, fmt.Errorf("%w: %v", ErrInvalidBootTime, err)
	}

	return true, nil
}

// IsWRPValid checks if a wrp is valid baased on the rules kept by the EventValidator.
func (e eventValidator) IsWRPValid(wrpWithTime queue.WrpWithTime) (bool, error) {
	msg := wrpWithTime.Message
	bootTime, err := GetWRPBootTime(msg)
	if err != nil || bootTime <= 0 {
		return false, ErrInvalidBootTime
	}
	// see if event found matches expected event type
	if !e.ValidateType(msg.Destination) {
		return false, fmt.Errorf("%w. Desired type: %s", ErrInvalidEventType, e.regex.String())
	}

	compareTime, err := e.GetWRPCompareTime(wrpWithTime)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrTimeParse, err)
	}

	if valid, err := e.timeValidation.IsTimeValid(compareTime); !valid {
		if e.calculateUsing == Birthdate {
			return false, fmt.Errorf("%w: %v", ErrInvalidBirthdate, err)
		}
		return false, fmt.Errorf("%w: %v", ErrInvalidBootTime, err)
	}

	return true, nil
}

// ValidateType validates that a destination string matches the type that the EventValidator is looking for
func (e eventValidator) ValidateType(dest string) bool {
	return e.regex.MatchString(dest)
}

// GetEventCompareTime gets the time used for comparison from an event
// depending on the TimeLocation of the EventValidator.
func (e eventValidator) GetEventCompareTime(event client.Event) (time.Time, error) {
	if e.calculateUsing == Boottime {
		bootTime, err := GetEventBootTime(event)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(bootTime, 0), nil
	}
	return time.Unix(0, event.BirthDate), nil
}

// GetWRPCompareTime gets the time used for comparison from a wrp
// depending on the TimeLocation of the EventValidator.
func (e eventValidator) GetWRPCompareTime(wrpWithTime queue.WrpWithTime) (time.Time, error) {
	if e.calculateUsing == Boottime {
		bootTime, err := GetWRPBootTime(wrpWithTime.Message)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(bootTime, 0), nil
	}
	return wrpWithTime.Beginning, nil
}
