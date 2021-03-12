package parsers

import (
	"time"

	"github.com/xmidt-org/glaukos/event/history"
	"github.com/xmidt-org/glaukos/event/queue"
)

// Validation is an interface for checking if wrp messages and events pass certain rules
type Validation interface {
	// ValidateEvent checks if an event is valid.
	ValidateEvent(history.Event) (bool, error)

	// ValidateWRP checks if a WrpWithTime is valid.
	ValidateWRP(queue.WrpWithTime) (bool, error)

	// ValidateType checks if a string matches the desired event type.
	ValidateType(string) bool

	// EventTime returns the time used for comparison from an event
	EventTime(history.Event) (time.Time, error)

	// WRPTime returns the time used for comparison from a WrpWithTime
	WRPTime(queue.WrpWithTime) (time.Time, error)
}
