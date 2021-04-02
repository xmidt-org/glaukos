package parsers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/interpreter"
)

func TestTimeElapsedCalculationErr(t *testing.T) {
	tests := []struct {
		description string
		timeElapsed float64
		event       interpreter.Event
		err         error
	}{
		{
			description: "nil underlying err",
			timeElapsed: 5.0,
		},
		{
			description: "underlying err not nil",
			err:         errors.New("test error"),
			timeElapsed: 5.0,
		},
		{
			description: "event not empty",
			event: interpreter.Event{
				TransactionUUID: "123",
			},
			timeElapsed: 5.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			calculationsErr := TimeElapsedCalculationErr{
				err:         tc.err,
				oldEvent:    tc.event,
				timeElapsed: tc.timeElapsed,
			}

			assert.Equal(tc.event, calculationsErr.Event())
			assert.Contains(calculationsErr.Error(), fmt.Sprint(tc.timeElapsed))
			if tc.err != nil {
				assert.Contains(calculationsErr.Error(), tc.err.Error())
			}

		})
	}
}
