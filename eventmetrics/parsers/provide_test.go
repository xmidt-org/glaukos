package parsers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheckTimeValidations(t *testing.T) {
	tests := []struct {
		description    string
		config         RebootParserConfig
		expectedConfig RebootParserConfig
	}{
		{
			description: "no changes",
			config: RebootParserConfig{
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
			expectedConfig: RebootParserConfig{
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
			config:      RebootParserConfig{},
			expectedConfig: RebootParserConfig{
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

func TestCreateEventValidator(t *testing.T) {
	configs := []RebootParserConfig{
		RebootParserConfig{},
		RebootParserConfig{
			ValidEventTypes: []string{"testEvent1", "testEvent2"},
			BootTimeValidator: TimeValidationConfig{
				ValidFrom: -1 * time.Hour,
				ValidTo:   time.Hour,
			},
			BirthdateValidator: TimeValidationConfig{
				ValidFrom: -1 * time.Hour,
				ValidTo:   time.Hour,
			},
			MetadataValidators:         []string{"key1"},
			MinBootDuration:            10 * time.Second,
			BirthdateAlignmentDuration: time.Hour,
		},
	}

	for _, config := range configs {
		validator := createEventValidator(config)
		assert.NotNil(t, validator)
	}
}
