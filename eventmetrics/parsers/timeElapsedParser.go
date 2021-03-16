package parsers

import (
	"time"

	"github.com/xmidt-org/glaukos/message"
)

// Validation is an interface for checking if wrp messages and events pass certain rules
type Validation interface {
	// ValidateEvent checks if an event is valid.
	ValidateEvent(message.Event) (bool, error)

	// ValidateType checks if a string matches the desired event type.
	ValidateType(string) bool

	// GetCompareTime returns the time used for comparison from an event
	GetCompareTime(message.Event) (time.Time, error)
}
