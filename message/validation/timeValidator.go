package validation

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrFutureDate  = errors.New("date is too far in the future")
	ErrPastDate    = errors.New("date is too far in the past")
	ErrNilTimeFunc = errors.New("currenttime function has not been set")
)

// TimeValidation sees if a given time is within the time frame it is set to validate
type TimeValidation interface {
	IsTimeValid(time.Time) (bool, error)
	CurrentTime() time.Time
}

// TimeValidator implements the TimeValidation interface
type TimeValidator struct {
	Current   func() time.Time
	ValidFrom time.Duration // should be a negative duration. If not, it will be changed to negative once IsTimeValid is called
	ValidTo   time.Duration
}

// IsTimeValid sees if a date is within a time validator's allowed time frame.
func (t TimeValidator) IsTimeValid(date time.Time) (bool, error) {
	if t.Current == nil {
		return false, ErrNilTimeFunc
	}

	if date.Before(time.Unix(0, 0)) || date.Equal(time.Unix(0, 0)) {
		return false, ErrPastDate
	}

	if t.ValidFrom.Seconds() > 0 {
		t.ValidFrom = -1 * t.ValidFrom
	}

	now := t.Current()
	pastTime := now.Add(t.ValidFrom)
	futureTime := now.Add(t.ValidTo)

	if !(pastTime.Before(date) || pastTime.Equal(date)) {
		return false, ErrPastDate
	}

	if !(futureTime.Equal(date) || futureTime.After(date)) {
		return false, ErrFutureDate
	}

	return true, nil
}

// CurrentTime returns the time that is given by the Current function
func (t TimeValidator) CurrentTime() time.Time {
	if t.Current != nil {
		return t.Current()
	}

	return time.Time{}
}

// TimeLocation is an enum to determine what should be used in timeElapsed calculations
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
