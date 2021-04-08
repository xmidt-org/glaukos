/**
 * Copyright 2021 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

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
