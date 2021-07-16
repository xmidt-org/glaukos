package enums

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationTypeString(t *testing.T) {
	tests := []struct {
		description    string
		validationType ValidationType
		expectedString string
	}{
		{
			description:    "valid type",
			validationType: BootTimeValidation,
			expectedString: BootTimeValidationStr,
		},
		{
			description:    "random type",
			validationType: ValidationType(2000),
			expectedString: UnknownValidationStr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			assert.Equal(tc.expectedString, tc.validationType.String())
		})
	}
}

func TestParseValidationType(t *testing.T) {
	tests := []struct {
		key          string
		expectedType ValidationType
	}{
		{
			key:          BootTimeValidationStr,
			expectedType: BootTimeValidation,
		},
		{
			key:          "abc-random-efg",
			expectedType: UnknownValidation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			assert := assert.New(t)
			valType := ParseValidationType(tc.key)
			assert.Equal(tc.expectedType, valType)
		})
	}
}
