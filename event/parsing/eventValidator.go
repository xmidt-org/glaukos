package parsing

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/xmidt-org/glaukos/event/queue"
)

type EventRule struct {
	Regex            string
	CalculateUsing   string
	DuplicateAllowed bool
}

type EventValidator struct {
	regex            *regexp.Regexp
	calculateUsing   TimeLocation
	timeValidator    TimeValidator
	duplicateAllowed bool
}

var (
	errInvalidEventType = errors.New("event type doesn't match")
	errInvalidRegex     = errors.New("invalid regex")
)

func NewEventValidator(rule EventRule, timeValidator TimeValidator) (EventValidator, error) {
	regex, err := regexp.Compile(rule.Regex)
	if err != nil {
		return EventValidator{}, fmt.Errorf("%w: %v", errInvalidRegex, err)
	}

	if timeValidator.CurrentTime == nil {
		timeValidator.CurrentTime = time.Now
	}

	timeLocation := ParseTimeLocation(rule.CalculateUsing)

	return EventValidator{
		regex:            regex,
		calculateUsing:   timeLocation,
		timeValidator:    timeValidator,
		duplicateAllowed: rule.DuplicateAllowed,
	}, nil
}

func (e *EventValidator) IsEventValid(event Event) (bool, error) {
	// see if event found matches expected event type
	if !e.ValidateType(event.Dest) {
		return false, fmt.Errorf("%w. Desired type: %s", errInvalidEventType, e.regex.String())
	}

	if e.calculateUsing == Birthdate {
		if valid, err := e.ValidateEventBirthdate(event); !valid {
			return false, fmt.Errorf("Invalid birthdate: %w", err)
		}
	} else {
		if valid, err := e.ValidateEventBootTime(event); !valid {
			return false, fmt.Errorf("Invalid boot-time: %w", err)
		}
	}

	return true, nil
}

func (e *EventValidator) IsWRPValid(wrpWithTime queue.WrpWithTime) (bool, error) {
	msg := wrpWithTime.Message
	// see if event found matches expected event type
	if !e.ValidateType(msg.Destination) {
		return false, fmt.Errorf("%w. Desired type: %s", errInvalidEventType, e.regex.String())
	}

	if e.calculateUsing == Birthdate {
		if valid, err := e.ValidateWRPBirthdate(wrpWithTime); !valid {
			return false, fmt.Errorf("Invalid birthdate: %w", err)
		}
	} else {
		if valid, err := e.ValidateWRPBootTime(wrpWithTime); !valid {
			return false, fmt.Errorf("Invalid boot-time: %w", err)
		}
	}

	return true, nil
}

func (e *EventValidator) ValidateType(dest string) bool {
	return e.regex.MatchString(dest)
}

func (e *EventValidator) ValidateEventBirthdate(event Event) (bool, error) {
	return e.timeValidator.IsDateValid(time.Unix(0, event.BirthDate))
}

func (e *EventValidator) ValidateEventBootTime(event Event) (bool, error) {
	bootTime, err := GetEventBootTime(event)
	if bootTime <= 0 {
		var parsingErr error
		if err != nil {
			parsingErr = err
		}
		return false, fmt.Errorf("%w. Parsed boot-time: %d, parsing err: %v", errPastDate, bootTime, parsingErr)
	}

	return e.timeValidator.IsDateValid(time.Unix(bootTime, 0))
}

func (e *EventValidator) ValidateWRPBootTime(wrpWithTime queue.WrpWithTime) (bool, error) {
	msg := wrpWithTime.Message
	bootTime, err := GetWRPBootTime(msg)
	if bootTime <= 0 {
		var parsingErr error
		if err != nil {
			parsingErr = err
		}
		return false, fmt.Errorf("%w. Parsed boot-time: %d, parsing err: %v", errPastDate, bootTime, parsingErr)
	}

	return e.timeValidator.IsDateValid(time.Unix(bootTime, 0))
}

func (e *EventValidator) ValidateWRPBirthdate(wrpWithTime queue.WrpWithTime) (bool, error) {
	return e.timeValidator.IsDateValid(wrpWithTime.Beginning)
}

func (e *EventValidator) DuplicateAllowed() bool {
	return e.duplicateAllowed
}

func (e *EventValidator) GetEventCompareTime(event Event) time.Time {
	if e.calculateUsing == Birthdate {
		return time.Unix(0, event.BirthDate)
	} else {
		bootTime, _ := GetEventBootTime(event)
		return time.Unix(bootTime, 0)
	}
}

func (e *EventValidator) GetWRPCompareTime(wrpWithTime queue.WrpWithTime) time.Time {
	if e.calculateUsing == Birthdate {
		return wrpWithTime.Beginning
	} else {
		bootTime, _ := GetWRPBootTime(wrpWithTime.Message)
		return time.Unix(bootTime, 0)
	}
}

func (e *EventValidator) Regex() *regexp.Regexp {
	return e.regex
}
