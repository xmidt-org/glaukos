package parsing

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/queue"
)

// EventRule is the config struct for event validation
type EventRule struct {
	Regex            string
	CalculateUsing   string
	DuplicateAllowed bool
	ValidFrom        time.Duration
}

// EventValidator implements a metricparser.EventValidator interface.
// Keeps a set of information/rules to see if an event or wrp fits these rules.
type EventValidator struct {
	regex            *regexp.Regexp
	calculateUsing   TimeLocation
	timeValidation   TimeValidation
	duplicateAllowed bool
}

var (
	errInvalidEventType = errors.New("event type doesn't match")
	errInvalidRegex     = errors.New("invalid regex")
)

// NewEventValidator creates a new EventValidator from an EventRule
func NewEventValidator(rule EventRule, defaultTimeValidation TimeValidation) (EventValidator, error) {
	regex, err := regexp.Compile(rule.Regex)
	if err != nil {
		return EventValidator{}, fmt.Errorf("%w: %v", errInvalidRegex, err)
	}

	var tv TimeValidation
	if rule.ValidFrom == 0 {
		tv = defaultTimeValidation
	} else {
		tv = TimeValidator{Current: time.Now, ValidFrom: rule.ValidFrom, ValidTo: time.Hour}
	}

	timeLocation := ParseTimeLocation(rule.CalculateUsing)

	return EventValidator{
		regex:            regex,
		calculateUsing:   timeLocation,
		timeValidation:   tv,
		duplicateAllowed: rule.DuplicateAllowed,
	}, nil
}

// IsEventValid checks if an event is valid based on the rules kept by the EventValidator.
func (e EventValidator) IsEventValid(event client.Event) (bool, error) {
	// see if event found matches expected event type
	if !e.ValidateType(event.Dest) {
		return false, fmt.Errorf("%w. Desired type: %s", errInvalidEventType, e.regex.String())
	}

	compareTime, err := e.GetEventCompareTime(event)
	if err != nil {
		return false, fmt.Errorf("Could not get time: %w", err)
	}

	if valid, err := e.timeValidation.IsTimeValid(compareTime); !valid {
		if e.calculateUsing == Birthdate {
			return false, fmt.Errorf("Invalid birthdate: %w", err)
		}
		return false, fmt.Errorf("Invalid boot-time: %w", err)
	}

	return true, nil
}

// IsWRPValid checks if a wrp is valid baased on the rules kept by the EventValidator.
func (e EventValidator) IsWRPValid(wrpWithTime queue.WrpWithTime) (bool, error) {
	msg := wrpWithTime.Message
	// see if event found matches expected event type
	if !e.ValidateType(msg.Destination) {
		return false, fmt.Errorf("%w. Desired type: %s", errInvalidEventType, e.regex.String())
	}

	compareTime, err := e.GetWRPCompareTime(wrpWithTime)
	if err != nil {
		return false, fmt.Errorf("Could not get time: %w", err)
	}

	if valid, err := e.timeValidation.IsTimeValid(compareTime); !valid {
		if e.calculateUsing == Birthdate {
			return false, fmt.Errorf("Invalid birthdate: %w", err)
		}
		return false, fmt.Errorf("Invalid boot-time: %w", err)
	}

	return true, nil
}

// ValidateType validates that a destination string matches the type that the EventValidator is looking for
func (e EventValidator) ValidateType(dest string) bool {
	return e.regex.MatchString(dest)
}

// DuplicateAllowed sees if the same type of event with the same boot-time is allowed.
func (e EventValidator) DuplicateAllowed() bool {
	return e.duplicateAllowed
}

// GetEventCompareTime gets the time used for comparison from an event
// depending on the TimeLocation of the EventValidator.
func (e EventValidator) GetEventCompareTime(event client.Event) (time.Time, error) {
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
func (e EventValidator) GetWRPCompareTime(wrpWithTime queue.WrpWithTime) (time.Time, error) {
	if e.calculateUsing == Boottime {
		bootTime, err := GetWRPBootTime(wrpWithTime.Message)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(bootTime, 0), nil
	}
	return wrpWithTime.Beginning, nil
}
