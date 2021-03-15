package validation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEventValidators(t *testing.T) {
	now, err := time.Parse(time.RFC3339Nano, "2021-03-02T18:00:01Z")
	assert.Nil(t, err)
	currTime := func() time.Time {
		return now
	}
	tests := []struct {
		description string
		rule        RuleConfig
		length      int
		expectedErr error
	}{

		{
			description: "Success",
			rule: RuleConfig{
				Regex:          ".*/online/.*",
				CalculateUsing: "Birthdate",
				ValidFrom:      -1 * time.Hour,
			},
			length: 3,
		},
		{
			description: "No Birthdate",
			rule: RuleConfig{
				Regex:          ".*/online/.*",
				CalculateUsing: "Boot-time",
				ValidFrom:      -1 * time.Hour,
			},
			length: 2,
		},
		{
			description: "Invalid Regex",
			rule: RuleConfig{
				Regex:          `'(?=.*\d)'`,
				CalculateUsing: "Boot-time",
				ValidFrom:      -1 * time.Hour,
			},
			expectedErr: errInvalidRegex,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			validators, err := NewEventValidators(tc.rule, time.Hour, currTime)
			if tc.expectedErr == nil {
				assert.Equal(tc.length, len(validators))
				assert.Nil(err)
			} else {
				assert.Equal(tc.expectedErr, err)
				assert.Nil(validators)
			}
		})
	}
}
