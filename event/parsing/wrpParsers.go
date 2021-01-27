package parsing

import (
	"errors"
	"regexp"
	"strconv"

	"github.com/xmidt-org/wrp-go/v3"
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

func GetDeviceID(destinationRegex *regexp.Regexp, destination string) (string, error) {
	match := destinationRegex.FindStringSubmatch(destination)
	if len(match) < 3 {
		return "", errors.New("error getting device ID from event")
	}

	return match[2], nil
}
