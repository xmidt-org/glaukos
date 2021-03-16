package validation

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/xmidt-org/glaukos/message"
)

// RuleConfig is the config struct for a rule
type RuleConfig struct {
	Regex          string
	CalculateUsing string
	ValidFrom      time.Duration
}

// rule implements a parsers.Validation interface.
// Keeps a set of information/rules to see if an event or wrp fits these rules.
type rule struct {
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

// NewRule creates a new rule from a RuleConfig
func NewRule(config RuleConfig, validTo time.Duration, currentTime func() time.Time) (*rule, error) {
	regex, err := regexp.Compile(config.Regex)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRegex, err)
	}

	tv := TimeValidator{Current: currentTime, ValidFrom: config.ValidFrom, ValidTo: validTo}

	timeLocation := ParseTimeLocation(config.CalculateUsing)

	return &rule{
		regex:          regex,
		calculateUsing: timeLocation,
		timeValidation: tv,
	}, nil
}

// IsEventValid checks if an event is valid using the information kept by the rule.
func (r *rule) ValidateEvent(event message.Event) (bool, error) {
	bootTime, err := event.BootTime()
	if err != nil || bootTime <= 0 {
		return false, ErrInvalidBootTime
	}

	// see if event found matches expected event type
	if !r.ValidateType(event.Destination) {
		return false, fmt.Errorf("%w. Desired type: %s", ErrInvalidEventType, r.regex.String())
	}

	compareTime, err := r.GetCompareTime(event)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrTimeParse, err)
	}

	if valid, err := r.timeValidation.IsTimeValid(compareTime); !valid {
		if r.calculateUsing == Birthdate {
			return false, fmt.Errorf("%w: %v", ErrInvalidBirthdate, err)
		}
		return false, fmt.Errorf("%w: %v", ErrInvalidBootTime, err)
	}

	return true, nil
}

// ValidateType validates that a destination string matches the type that the rule is looking for
func (r *rule) ValidateType(dest string) bool {
	return r.regex.MatchString(dest)
}

// GetCompareTime gets the time used for comparison from an event
// depending on the TimeLocation of the rule.
func (r *rule) GetCompareTime(event message.Event) (time.Time, error) {
	if r.calculateUsing == Boottime {
		bootTime, err := event.BootTime()
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(bootTime, 0), nil
	}
	return time.Unix(0, event.Birthdate), nil
}
