// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

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

	tests := []struct {
		description  string
		location     TimeLocation
		expectedTime time.Time
		event        interpreter.Event
	}{
		{
			description:  "Valid Birthdate",
			location:     Birthdate,
			expectedTime: birthdate,
			event: interpreter.Event{
				Birthdate: birthdate.UnixNano(),
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(bootTime.Unix()),
				},
			},
		},
		{
			description:  "Valid Boot-time",
			location:     Boottime,
			expectedTime: bootTime,
			event: interpreter.Event{
				Birthdate: birthdate.UnixNano(),
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(bootTime.Unix()),
				},
			},
		},
		{
			description:  "Invalid Birthdate",
			location:     Birthdate,
			expectedTime: time.Time{},
			event: interpreter.Event{
				Metadata: map[string]string{
					interpreter.BootTimeKey: fmt.Sprint(bootTime.Unix()),
				},
			},
		},
		{
			description:  "Invalid Boot-time",
			location:     Boottime,
			expectedTime: time.Time{},
			event: interpreter.Event{
				Birthdate: birthdate.UnixNano(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			time := ParseTime(tc.event, tc.location)
			assert.True(tc.expectedTime.Equal(time))
			assert.Nil(err)
		})
	}
}
