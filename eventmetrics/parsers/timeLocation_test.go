package parsers

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/interpreter"
)

func TestParseTimeLocation(t *testing.T) {
	tests := []struct {
		testLocation     string
		expectedLocation TimeLocation
	}{
		{
			testLocation:     "Birthdate",
			expectedLocation: Birthdate,
		},
		{
			testLocation:     "Boot-time",
			expectedLocation: Boottime,
		},
		{
			testLocation:     "birthdate",
			expectedLocation: Birthdate,
		},
		{
			testLocation:     "boot-time",
			expectedLocation: Boottime,
		},
		{
			testLocation:     "random",
			expectedLocation: Birthdate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testLocation, func(t *testing.T) {
			assert := assert.New(t)
			res := ParseTimeLocation(tc.testLocation)
			assert.Equal(tc.expectedLocation, res)
		})
	}
}

func TestParseTime(t *testing.T) {
	birthdate, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	bootTime, err := time.Parse(time.RFC3339Nano, "2021-03-01T18:00:01Z")
	assert.Nil(t, err)
	event := interpreter.Event{
		Birthdate: birthdate.UnixNano(),
		Metadata: map[string]string{
			interpreter.BootTimeKey: fmt.Sprint(bootTime.Unix()),
		},
	}

	tests := []struct {
		description  string
		expectedTime int64
	}{
		{
			description:  "Birthdate",
			expectedTime: birthdate.UnixNano(),
		},
		{
			description:  "Boot-time",
			expectedTime: bootTime.Unix(),
		},
		{
			description:  "Random",
			expectedTime: birthdate.UnixNano(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			time, err := ParseTime(event, tc.description)
			assert.Equal(tc.expectedTime, time)
			assert.Nil(err)
		})
	}
}
