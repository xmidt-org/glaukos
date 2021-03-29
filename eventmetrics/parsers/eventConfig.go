package parsers

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/xmidt-org/interpreter"
	"github.com/xmidt-org/interpreter/validation"
)

var (
	errInvalidRegex = errors.New("invalid regex")
)

type TimeFunc func() time.Time

type EventInfo struct {
	Regex          *regexp.Regexp
	CalculateUsing TimeLocation
	Validator      validation.Validator
}

type EventConfig struct {
	Regex          string
	CalculateUsing string
	ValidFrom      time.Duration
}

// NewEventInfo creates a new EventInfo from an EventConfig.
// Will return an empty EventInfo and an error if the regex is invalid.
func NewEventInfo(config EventConfig, current TimeFunc) (EventInfo, error) {
	regex, err := regexp.Compile(config.Regex)
	if err != nil {
		return EventInfo{}, errInvalidRegex
	}

	timeValidator := validation.TimeValidator{
		ValidFrom: config.ValidFrom,
		ValidTo:   time.Hour,
		Current:   current,
	}

	// destination and boot-time validators are needed for all events.
	validators := []validation.Validator{
		validation.DestinationValidator(regex),
		validation.BootTimeValidator(timeValidator),
	}

	timeLocation := ParseTimeLocation(config.CalculateUsing)
	// If birthdate is used in calculations, add a birthdate validator.
	if timeLocation == Birthdate {
		validators = append(validators, validation.BirthdateValidator(timeValidator))
	}

	return EventInfo{
		Regex:          regex,
		CalculateUsing: timeLocation,
		Validator:      validation.Validators(validators),
	}, nil
}

// Valid implements the validation.Validator interface.
// If an EventInfo's validator is nil, it means there is no validation needed, so the event
// is valid by default.
func (e EventInfo) Valid(event interpreter.Event) (bool, error) {
	if e.Validator == nil {
		return true, nil
	}
	return e.Validator.Valid(event)
}

// TimeLocation is an enum to determine which timestamp should be used in timeElapsed calculations
type TimeLocation int

const (
	Birthdate TimeLocation = iota
	Boottime
)

var (
	timeLocationUnmarshal = map[string]TimeLocation{
		"birthdate": Birthdate,
		"boot-time": Boottime,
	}
)

// ParseTimeLocation returns the TimeLocation enum when given a string.
func ParseTimeLocation(location string) TimeLocation {
	location = strings.ToLower(location)
	if value, ok := timeLocationUnmarshal[location]; ok {
		return value
	}
	return Birthdate
}

// ParseTime gets the time from the proper location of an Event
func ParseTime(e interpreter.Event, location TimeLocation) time.Time {
	if location == Birthdate {
		if e.Birthdate > 0 {
			return time.Unix(0, e.Birthdate)
		} else {
			return time.Time{}
		}

	}

	if bootTime, err := e.BootTime(); err == nil {
		return time.Unix(bootTime, 0)
	} else {
		return time.Time{}
	}
}

// SessionType is an enum to determine which session an event should be searched from
type SessionType int

const (
	Previous SessionType = iota
	Current
)

var (
	sessionTypeUnmarshal = map[string]SessionType{
		"previous": Previous,
		"current":  Current,
	}
)

// ParseSessionType returns the SessionType enum when given a string.
func ParseSessionType(session string) SessionType {
	session = strings.ToLower(session)
	if value, ok := sessionTypeUnmarshal[session]; ok {
		return value
	}
	return Previous
}
