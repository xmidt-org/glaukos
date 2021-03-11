package parsing

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/wrp-go/v3"
)

const (
	bootTimeKey = "/boot-time"
)

var (
	ErrParseDeviceID = errors.New("error getting device ID from event")

	ErrBootTimeParse    = errors.New("unable to parse boot-time")
	ErrBootTimeNotFound = errors.New("boot-time not found")
)

// GetWRPBootTime grabs the boot-time from a wrp.Message's metadata.
func GetWRPBootTime(msg wrp.Message) (int64, error) {
	var bootTime int64
	var err error

	bootTimeStr, ok := GetMetadataValue(bootTimeKey, msg.Metadata)

	if ok {
		bootTime, err = strconv.ParseInt(bootTimeStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %v", ErrBootTimeParse, err)
		}
	} else {
		err = ErrBootTimeNotFound
	}

	return bootTime, err
}

// GetEventBootTime grabs the boot-time from a Event's metadata.
func GetEventBootTime(msg client.Event) (int64, error) {
	var bootTime int64
	var err error
	bootTimeStr, ok := GetMetadataValue(bootTimeKey, msg.Metadata)

	if ok {
		bootTime, err = strconv.ParseInt(bootTimeStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %v", ErrBootTimeParse, err)
		}
	} else {
		err = ErrBootTimeNotFound
	}

	return bootTime, err
}

// GetDeviceID grabs the device id from a given destination string.
func GetDeviceID(destinationRegex *regexp.Regexp, destination string) (string, error) {
	match := destinationRegex.FindStringSubmatch(destination)
	if len(match) < 3 {
		return "", ErrParseDeviceID
	}

	return match[2], nil
}

// GetValidBirthDate attempts to get the birthdate from the payload.
// If it doesn't exist, the current time is returned.
// If the birthdate is too old or too far in the future, 0 is returned.
func GetValidBirthDate(currTime func() time.Time, payload []byte) (time.Time, error) {
	now := currTime()
	birthDate, ok := getBirthDate(payload)
	if !ok {
		birthDate = now
	}

	// check if birthdate is within the last 12 hours and the next hour
	if valid, err := IsDateValid(currTime, 12*time.Hour, time.Hour, birthDate); !valid {
		return time.Time{}, err
	}

	return birthDate, nil
}

func getBirthDate(payload []byte) (time.Time, bool) {
	p := make(map[string]interface{})
	if len(payload) == 0 {
		return time.Time{}, false
	}
	err := json.Unmarshal(payload, &p)
	if err != nil {
		return time.Time{}, false
	}

	// parse the time from the payload
	timeString, ok := p["ts"].(string)
	if !ok {
		return time.Time{}, false
	}
	birthDate, err := time.Parse(time.RFC3339Nano, timeString)
	if err != nil {
		return time.Time{}, false
	}
	return birthDate, true
}

// Sees if a date is within a certain time frame.
// PastBuffer should be a positive duration.
func IsDateValid(currTime func() time.Time, pastBuffer time.Duration, futureBuffer time.Duration, date time.Time) (bool, error) {
	if date.Before(time.Unix(0, 0)) || date.Equal(time.Unix(0, 0)) {
		return false, ErrPastDate
	}

	if pastBuffer.Seconds() < 0 {
		pastBuffer = -1 * pastBuffer
	}

	now := currTime()
	pastTime := now.Add(-1 * pastBuffer)
	futureTime := now.Add(futureBuffer)

	if !(pastTime.Before(date) || pastTime.Equal(date)) {
		return false, ErrPastDate
	}

	if !(futureTime.Equal(date) || futureTime.After(date)) {
		return false, ErrFutureDate
	}

	return true, nil
}

// Checks a map for a specific key, allowing for keys with or without forward-slash
func GetMetadataValue(key string, metadata map[string]string) (string, bool) {
	value, found := metadata[key]
	if !found {
		value, found = metadata[strings.Trim(key, "/")]
	}

	return value, found
}
