package parsing

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/goph/emperror"
	"github.com/xmidt-org/wrp-go/v3"
)

var (
	errParseDeviceID   = errors.New("error getting device ID from event")
	errFutureBirthDate = errors.New("birthdate is too far in the future")
	errPastBirthDate   = errors.New("birthdate is too far in the past")
)

// GetWRPBootTime grabs the boot-time from a wrp.Message's metadata.
func GetWRPBootTime(msg wrp.Message) (int64, error) {
	var bootTime int64
	var err error
	if bootTimeStr, ok := msg.Metadata[bootTimeKey]; ok {
		bootTime, err = strconv.ParseInt(bootTimeStr, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return bootTime, nil
}

// GetEventBootTime grabs the boot-time from a Event's metadata.
func GetEventBootTime(msg Event) (int64, error) {
	var bootTime int64
	var err error
	if bootTimeStr, ok := msg.Metadata[bootTimeKey]; ok {
		bootTime, err = strconv.ParseInt(bootTimeStr, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return bootTime, nil
}

// GetDeviceID grabs the device id from a given destination string.
func GetDeviceID(destinationRegex *regexp.Regexp, destination string) (string, error) {
	match := destinationRegex.FindStringSubmatch(destination)
	if len(match) < 3 {
		return "", errParseDeviceID
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

	// for time skew reasons
	if birthDate.After(now.Add(time.Hour)) {
		return time.Time{}, emperror.WrapWith(errFutureBirthDate, "invalid birthdate", "birthdate", birthDate.String())
	}

	// check if birth date is too old (past a week)
	// TODO: should this be configurable?
	if birthDate.Before(now.Add(-168 * time.Hour)) {
		return time.Time{}, emperror.WrapWith(errPastBirthDate, "invalid birthdate", "birthdate", birthDate.String())
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
