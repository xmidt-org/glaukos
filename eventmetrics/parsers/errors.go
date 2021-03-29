package parsers

import (
	"fmt"

	"github.com/xmidt-org/interpreter"
)

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

func (t TimeElapsedCalculationErr) Event() interpreter.Event {
	return t.oldEvent
}
