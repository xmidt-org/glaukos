package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSessionType(t *testing.T) {
	tests := []struct {
		testStr      string
		expectedType SessionType
	}{
		{
			testStr:      "Previous",
			expectedType: Previous,
		},
		{
			testStr:      "Current",
			expectedType: Current,
		},
		{
			testStr:      "previous",
			expectedType: Previous,
		},
		{
			testStr:      "current",
			expectedType: Current,
		},
		{
			testStr:      "random",
			expectedType: Previous,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testStr, func(t *testing.T) {
			assert := assert.New(t)
			res := ParseSessionType(tc.testStr)
			assert.Equal(tc.expectedType, res)
		})
	}
}
