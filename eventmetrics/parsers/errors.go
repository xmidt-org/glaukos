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
	"fmt"
	"strings"

	"github.com/xmidt-org/interpreter"
)

// TimeElapsedCalculationErr is an error thrown when the time elapsed calculations result in an error.
// Contains the comparison event found in the events history (if calculations have progressed that far),
// the timeElapsed (if calculations have progressed that far), the prometheus reason label associated with the error.
type TimeElapsedCalculationErr struct {
	err         error
	timeElapsed float64
	errLabel    string
	oldEvent    interpreter.Event
}

func (t TimeElapsedCalculationErr) Error() string {
	if t.err != nil {
		return fmt.Sprintf("invalid time elapsed: %v. time calculated: %f", t.err, t.timeElapsed)
	}
	return fmt.Sprintf("invalid time elapsed. time calculated: %f", t.timeElapsed)
}

func (t TimeElapsedCalculationErr) Unwrap() error {
	return t.err
}

// ErrorLabel returns the prometheus error label associated with this error
func (t TimeElapsedCalculationErr) ErrorLabel() string {
	if len(t.errLabel) > 0 {
		return strings.ReplaceAll(t.errLabel, " ", "_")
	}
	return "unknown"
}

// Event implements the ErrorWithEvent interface and returns the event found in the history that caused the error.
func (t TimeElapsedCalculationErr) Event() interpreter.Event {
	return t.oldEvent
}
