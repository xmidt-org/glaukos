// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package enums

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationTypeString(t *testing.T) {
	tests := []struct {
		description    string
		validationType EventValidationType
		expectedString string
	}{
		{
			description:    "valid type",
			validationType: BootTimeValidation,
			expectedString: BootTimeValidationStr,
		},
		{
			description:    "random type",
			validationType: EventValidationType(2000),
			expectedString: UnknownEventValidationStr,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			assert.Equal(tc.expectedString, tc.validationType.String())
		})
	}
}

func TestEventValidationUnmarshalText(t *testing.T) {
	tests := []struct {
		key          string
		expectedType EventValidationType
	}{
		{
			key:          BootTimeValidationStr,
			expectedType: BootTimeValidation,
		},
		{
			key:          "abc-random-efg",
			expectedType: UnknownEventValidation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			assert := assert.New(t)
			valType := EventValidationType(1000)
			valType.UnmarshalText([]byte(tc.key))
			assert.Equal(tc.expectedType, valType)
		})
	}
}
