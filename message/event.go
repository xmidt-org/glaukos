package message

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xmidt-org/wrp-go/v3"
)

const (
	BootTimeKey = "/boot-time"
)

var (
	ErrParseDeviceID    = errors.New("error getting device ID from event")
	ErrBirthdateParse   = errors.New("unable to parse birthdate from payload")
	ErrBootTimeParse    = errors.New("unable to parse boot-time")
	ErrBootTimeNotFound = errors.New("boot-time not found")
)

// Event is the struct that contains the wrp.Message fields along with the birthdate
// that is parsed from the payload.
type Event struct {
	MsgType         int               `json:"msg_type"`
	Source          string            `json:"source"`
	Destination     string            `json:"dest,omitempty"`
	TransactionUUID string            `json:"transaction_uuid,omitempty"`
	ContentType     string            `json:"content_type,omitempty"`
	Metadata        map[string]string `json:"metadata"`
	Payload         string            `json:"payload,omitempty"`
	Birthdate       int64             `json:"birth_date"`
	PartnerIDs      []string          `json:"partner_ids,omitempty"`
}

// NewEvent creates an Event from a wrp.Message and also parses the Birthdate from the
// message payload. A new Event will always be returned from this function, but if the
// birthdate cannot be parsed from the payload, it will return an error along with the Event created.
func NewEvent(msg wrp.Message) (Event, error) {
	var err error
	event := Event{
		MsgType:         int(msg.MessageType()),
		Source:          msg.Source,
		Destination:     msg.Destination,
		TransactionUUID: msg.TransactionUUID,
		ContentType:     msg.ContentType,
		Metadata:        msg.Metadata,
		Payload:         string(msg.Payload),
		PartnerIDs:      msg.PartnerIDs,
	}

	if birthdate, ok := getBirthDate(msg.Payload); ok {
		event.Birthdate = birthdate.UnixNano()
	} else {
		err = ErrBirthdateParse
	}

	return event, err
}

// GetMetadataValue checks the metadata map for a specific key,
// allowing for keys with or without forward-slash.
func (e Event) GetMetadataValue(key string) (string, bool) {
	value, found := e.Metadata[key]
	if !found {
		value, found = e.Metadata[strings.Trim(key, "/")]
	}

	return value, found
}

// BootTime parses the boot-time from an event, returning an
// error if the boot-time doesn't exist or cannot be parsed.
func (e Event) BootTime() (int64, error) {
	bootTimeStr, ok := e.GetMetadataValue(BootTimeKey)
	if !ok {
		return 0, ErrBootTimeNotFound
	}

	bootTime, err := strconv.ParseInt(bootTimeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrBootTimeParse, err)
	}

	return bootTime, err
}

// DeviceID gets the device id based on a regex
func (e *Event) DeviceID(regex *regexp.Regexp) (string, error) {
	match := regex.FindStringSubmatch(e.Destination)
	if len(match) < 3 {
		return "", ErrParseDeviceID
	}

	return match[2], nil
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
