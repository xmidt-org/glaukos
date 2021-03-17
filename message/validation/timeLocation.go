package validation

import (
	"strings"

	"github.com/xmidt-org/glaukos/message"
)

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

// ParseTime gets the timestamp from the proper location of an Event
func ParseTime(e message.Event, locationStr string) (int64, error) {
	location := ParseTimeLocation(locationStr)

	if location == Birthdate {
		return e.Birthdate, nil
	}
	return e.BootTime()
}
