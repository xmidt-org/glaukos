package enums

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCycleValidationTypeString(t *testing.T) {
	tests := []struct {
		description    string
		validationType CycleValidationType
		expectedString string
	}{
		{
			description:    "valid type",
			validationType: ConsistentMetadataValidation,
			expectedString: ConsistentMetadataValidationStr,
		},
		{
			description:    "random type",
			validationType: CycleValidationType(2000),
			expectedString: UnknownCycleValidationStr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			assert.Equal(tc.expectedString, tc.validationType.String())
		})
	}
}

func TestCycleValidationUnmarshalText(t *testing.T) {
	tests := []struct {
		key          string
		expectedType CycleValidationType
	}{
		{
			key:          ConsistentMetadataValidationStr,
			expectedType: ConsistentMetadataValidation,
		},
		{
			key:          "abc-random-efg",
			expectedType: UnknownCycleValidation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			assert := assert.New(t)
			valType := CycleValidationType(1000)
			valType.UnmarshalText([]byte(tc.key))
			assert.Equal(tc.expectedType, valType)
		})
	}
}
