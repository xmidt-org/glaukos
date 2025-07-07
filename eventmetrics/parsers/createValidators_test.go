// SPDX-FileCopyrightText: 2025 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/glaukos/eventmetrics/parsers/enums"
	"github.com/xmidt-org/interpreter/history"
)

func TestCreateEventValidator(t *testing.T) {
	tests := []struct {
		key         enums.EventValidationType
		expectedErr error
	}{
		{
			key:         enums.UnknownEventValidation,
			expectedErr: errNonExistentKey,
		},
		{key: enums.BootTimeValidation},
		{key: enums.BirthdateValidation},
		{key: enums.MinBootDurationValidation},
		{key: enums.BirthdateAlignmentValidation},
		{key: enums.ValidEventTypeValidation},
		{key: enums.ConsistentDeviceIDValidation},
	}

	for _, tc := range tests {
		t.Run(tc.key.String(), func(t *testing.T) {
			assert := assert.New(t)
			config := EventValidationConfig{
				Key: tc.key,
			}
			validator, err := createEventValidator(config)
			if tc.expectedErr != nil {
				assert.Nil(validator)
			}
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestCreateCycleValidator(t *testing.T) {
	tests := []struct {
		key         enums.CycleValidationType
		expectedErr error
	}{
		{
			key:         enums.UnknownCycleValidation,
			expectedErr: errNonExistentKey,
		},
		{key: enums.ConsistentMetadataValidation},
		{key: enums.UniqueTransactionIDValidation},
		{key: enums.SessionOnlineValidation},
		{key: enums.SessionOfflineValidation},
		{key: enums.EventOrderValidation},
	}

	for _, tc := range tests {
		t.Run(tc.key.String(), func(t *testing.T) {
			assert := assert.New(t)
			config := CycleValidationConfig{
				Key: tc.key,
			}
			validator, err := createCycleValidator(config)
			if tc.expectedErr != nil {
				assert.Nil(validator)
			}
			assert.Equal(tc.expectedErr, err)
		})
	}
}

func TestCreateCycleValidators(t *testing.T) {
	tests := []struct {
		description string
		cycleType   enums.CycleType
		configs     []CycleValidationConfig
		expectedLen int
		expectedErr error
	}{
		{
			description: "boot-time cycle type",
			cycleType:   enums.BootTime,
			configs: []CycleValidationConfig{
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataValidation,
					CycleType: "boot-time",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataValidation,
					CycleType: "reboot",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataValidation,
					CycleType: "random-cycle",
				},
			},
			expectedLen: 2,
		},
		{
			description: "reboot cycle type",
			cycleType:   enums.Reboot,
			configs: []CycleValidationConfig{
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataValidation,
					CycleType: "boot-time",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataValidation,
					CycleType: "reboot",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataValidation,
					CycleType: "random-cycle",
				},
			},
			expectedLen: 1,
		},
		{
			description: "wrong validation key",
			cycleType:   enums.Reboot,
			configs: []CycleValidationConfig{
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataValidation,
					CycleType: "boot-time",
				},
				CycleValidationConfig{
					Key:       enums.UnknownCycleValidation,
					CycleType: "reboot",
				},
				CycleValidationConfig{
					Key:       enums.ConsistentMetadataValidation,
					CycleType: "random-cycle",
				},
			},
			expectedLen: 0,
			expectedErr: errNonExistentKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			validator, err := createCycleValidators(tc.configs, tc.cycleType)
			if validator != nil {
				validators := validator.(history.CycleValidators)
				assert.Equal(tc.expectedLen, len(validators))
			}
			assert.Equal(tc.expectedErr, err)
		})
	}
}
