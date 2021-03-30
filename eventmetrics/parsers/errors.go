package parsers

import (
	"fmt"

	"github.com/xmidt-org/interpreter"
)

// TimeElapsedCalculationErr is an error thrown when the time elapsed calculations result in an invalid number.
// Contains the comparison event found in the events history.
type TimeElapsedCalculationErr struct {
	err         error
	timeElapsed float64
	oldEvent    interpreter.Event
}

func (t TimeElapsedCalculationErr) Error() string {
	if t.err != nil {
		return fmt.Sprintf("invalid time elapsed: %v. time calculated: %f", t.err, t.timeElapsed)
	}
	return fmt.Sprintf("invalid time elapsed. time calculated: %f", t.timeElapsed)
}

// Event implements the ErrorWithEvent interface and returns the event found in the history that caused the error.
func (t TimeElapsedCalculationErr) Event() interpreter.Event {
	return t.oldEvent
}
