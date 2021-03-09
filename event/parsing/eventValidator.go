package parsing

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/queue"
)

type EventRule struct {
	Regex            string
	CalculateUsing   string
	DuplicateAllowed bool
	ValidFrom        time.Duration
}

type eventValidator struct {
	regex            *regexp.Regexp
	calculateUsing   TimeLocation
	timeValidator    TimeValidator
	duplicateAllowed bool
}

var (
	errInvalidEventType = errors.New("event type doesn't match")
	errInvalidRegex     = errors.New("invalid regex")
)

func NewEventValidator(rule EventRule, defaultTimeValidator TimeValidator) (eventValidator, error) {
	regex, err := regexp.Compile(rule.Regex)
	if err != nil {
		return eventValidator{}, fmt.Errorf("%w: %v", errInvalidRegex, err)
	}

	var tv TimeValidator
	if rule.ValidFrom == 0 {
		tv = defaultTimeValidator
	} else {
		tv = TimeValidation{CurrentTime: time.Now, ValidFrom: rule.ValidFrom, ValidTo: time.Hour}
	}

	timeLocation := ParseTimeLocation(rule.CalculateUsing)

	return eventValidator{
		regex:            regex,
		calculateUsing:   timeLocation,
		timeValidator:    tv,
		duplicateAllowed: rule.DuplicateAllowed,
	}, nil
}

func (e eventValidator) IsEventValid(event client.Event) (bool, error) {
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

func (e eventValidator) IsWRPValid(wrpWithTime queue.WrpWithTime) (bool, error) {
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

func (e eventValidator) ValidateType(dest string) bool {
	return e.regex.MatchString(dest)
}

func (e eventValidator) ValidateEventBirthdate(event client.Event) (bool, error) {
	return e.timeValidator.IsTimeValid(time.Unix(0, event.BirthDate))
}

func (e eventValidator) ValidateEventBootTime(event client.Event) (bool, error) {
	bootTime, err := GetEventBootTime(event)
	if bootTime <= 0 {
		return false, fmt.Errorf("%w. Parsed boot-time: %d, parsing err: %v", errPastDate, bootTime, err)
	}

	return e.timeValidator.IsTimeValid(time.Unix(bootTime, 0))
}

func (e eventValidator) ValidateWRPBootTime(wrpWithTime queue.WrpWithTime) (bool, error) {
	msg := wrpWithTime.Message
	bootTime, err := GetWRPBootTime(msg)
	if bootTime <= 0 {
		return false, fmt.Errorf("%w. Parsed boot-time: %d, parsing err: %v", errPastDate, bootTime, err)
	}

	return e.timeValidator.IsTimeValid(time.Unix(bootTime, 0))
}

func (e eventValidator) ValidateWRPBirthdate(wrpWithTime queue.WrpWithTime) (bool, error) {
	return e.timeValidator.IsTimeValid(wrpWithTime.Beginning)
}

func (e eventValidator) DuplicateAllowed() bool {
	return e.duplicateAllowed
}

func (e eventValidator) GetEventCompareTime(event client.Event) time.Time {
	if e.calculateUsing == Boottime {
		bootTime, _ := GetEventBootTime(event)
		return time.Unix(bootTime, 0)
	}
	return time.Unix(0, event.BirthDate)
}

func (e eventValidator) GetWRPCompareTime(wrpWithTime queue.WrpWithTime) time.Time {
	if e.calculateUsing == Boottime {
		bootTime, _ := GetWRPBootTime(wrpWithTime.Message)
		return time.Unix(bootTime, 0)
	}
	return wrpWithTime.Beginning
}
