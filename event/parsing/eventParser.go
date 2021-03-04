package parsing

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/xmidt-org/glaukos/event/queue"
)

type EventConfig struct {
	Regex               string
	CalculateUsing      string
	BirthdateValidation TimeValidator
	BootTimeValidation  TimeValidator
	RepeatAllowed       bool
}

type EventValidator struct {
	regex               *regexp.Regexp
	calculateUsing      TimeLocation
	birthdateValidation TimeValidator
	bootTimeValidation  TimeValidator
	repeatAllowed       bool
}

var (
	errEventNotFound = errors.New("event not found")
)

func (e *EventValidator) IsEventValid(event Event, expectedType *regexp.Regexp, currTime func() time.Time) (bool, error) {
	// see if event found matches expected event type
	if !e.regex.MatchString(event.Dest) {
		return false, fmt.Errorf("%w. Type: %s", errEventNotFound, e.regex.String())
	}

	// check if boot-time is valid
	bootTime, err := GetEventBootTime(event)
	if bootTime <= 0 {
		var parsingErr error
		if err != nil {
			parsingErr = err
		}
		return false, fmt.Errorf("%w. Parsed boot-time: %d, parsing err: %v", errPastDate, bootTime, parsingErr)
	}

	if valid, err := e.bootTimeValidation.IsDateValid(time.Unix(bootTime, 0)); !valid {
		return false, fmt.Errorf("Invalid boot-time: %w", err)
	}

	// see if birthdate is valid
	if valid, err := e.birthdateValidation.IsDateValid(time.Unix(0, event.BirthDate)); !valid {
		return false, fmt.Errorf("Invalid birthdate: %w", err)
	}

	return true, nil
}

func (e *EventValidator) GetCompareTimeEvent(event Event) time.Time {
	if e.calculateUsing == Birthdate {
		return time.Unix(0, event.BirthDate)
	} else {
		bootTime, _ := GetEventBootTime(event)
		return time.Unix(bootTime, 0)
	}
}

func (e *EventValidator) GetCompareTimeWRP(wrpWithTime queue.WrpWithTime) time.Time {
	if e.calculateUsing == Birthdate {
		return wrpWithTime.Beginning
	} else {
		bootTime, _ := GetWRPBootTime(wrpWithTime.Message)
		return time.Unix(bootTime, 0)
	}
}
