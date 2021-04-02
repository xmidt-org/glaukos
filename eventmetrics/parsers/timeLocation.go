package parsers

import (
	"strings"
	"time"

	"github.com/xmidt-org/interpreter"
)

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
