package parsing

import (
	"strings"
	"time"
)

type TimeValidator interface {
	IsTimeValid(time.Time) (bool, error)
}

type TimeValidation struct {
	CurrentTime func() time.Time
	ValidFrom   time.Duration
	ValidTo     time.Duration
}

// IsTimeValid sees if a date is within a time validator's allowed time frame.
func (t TimeValidation) IsTimeValid(date time.Time) (bool, error) {
	if t.CurrentTime == nil {
		t.CurrentTime = time.Now
	}
	return isTimeValid(t.CurrentTime, t.ValidFrom, t.ValidTo, date)
}

// Sees if a date is within a certain time frame.
// PastBuffer should be a negative duration.
func isTimeValid(currTime func() time.Time, pastBuffer time.Duration, futureBuffer time.Duration, date time.Time) (bool, error) {
	if date.Before(time.Unix(0, 0)) || date.Equal(time.Unix(0, 0)) {
		return false, errPastDate
	}

	if pastBuffer.Seconds() > 0 {
		pastBuffer = -1 * pastBuffer
	}

	now := currTime()
	pastTime := now.Add(pastBuffer)
	futureTime := now.Add(futureBuffer)

	if !(pastTime.Before(date) || pastTime.Equal(date)) {
		return false, errPastDate
	}

	if !(futureTime.Equal(date) || futureTime.After(date)) {
		return false, errFutureDate
	}

	return true, nil
}

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

// ParseEventType returns the enum when given a string.
func ParseTimeLocation(location string) TimeLocation {
	location = strings.ToLower(location)
	if value, ok := timeLocationUnmarshal[location]; ok {
		return value
	}
	return Birthdate
}
