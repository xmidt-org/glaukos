// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package enums

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCycleType(t *testing.T) {
	tests := []struct {
		testStr      string
		expectedType CycleType
	}{
		{
			testStr:      "boot-time",
			expectedType: BootTime,
		},
		{
			testStr:      "reboot",
			expectedType: Reboot,
		},
		{
			testStr:      "BoOt-Time",
			expectedType: BootTime,
		},
		{
			testStr:      "Reboot",
			expectedType: Reboot,
		},
		{
			testStr:      "random",
			expectedType: BootTime,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testStr, func(t *testing.T) {
			assert := assert.New(t)
			res := ParseCycleType(tc.testStr)
			assert.Equal(tc.expectedType, res)
		})
	}
}
