package parsers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckTimeValidations(t *testing.T) {
	tests := []struct {
		description    string
		config         EventValidationConfig
		expectedConfig EventValidationConfig
	}{
		{
			description: "no changes",
			config: EventValidationConfig{
				BootTimeValidator: TimeValidationConfig{
					ValidFrom:    -1 * time.Hour,
					ValidTo:      2 * time.Hour,
					MinValidYear: 2015,
				},
				BirthdateValidator: TimeValidationConfig{
					ValidFrom:    -1 * time.Hour,
					ValidTo:      2 * time.Hour,
					MinValidYear: 2015,
				},
				MinBootDuration:            5 * time.Second,
				BirthdateAlignmentDuration: 2 * time.Minute,
			},
			expectedConfig: EventValidationConfig{
				BootTimeValidator: TimeValidationConfig{
					ValidFrom:    -1 * time.Hour,
					ValidTo:      2 * time.Hour,
					MinValidYear: 2015,
				},
				BirthdateValidator: TimeValidationConfig{
					ValidFrom:    -1 * time.Hour,
					ValidTo:      2 * time.Hour,
					MinValidYear: 2015,
				},
				MinBootDuration:            5 * time.Second,
				BirthdateAlignmentDuration: 2 * time.Minute,
			},
		},
		{
			description: "defaults",
			config:      EventValidationConfig{},
			expectedConfig: EventValidationConfig{
				BootTimeValidator: TimeValidationConfig{
					ValidFrom: defaultValidFrom,
					ValidTo:   defaultValidTo,
				},
				BirthdateValidator: TimeValidationConfig{
					ValidFrom: defaultValidFrom,
					ValidTo:   defaultValidTo,
				},
				MinBootDuration:            defaultMinBootDuration,
				BirthdateAlignmentDuration: defaultBirthdateAlignmentDuration,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			resultingConfig := checkTimeValidations(tc.config)
			assert.Equal(tc.expectedConfig, resultingConfig)
		})
	}
}
