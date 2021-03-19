package validation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidEventErr(t *testing.T) {
	testErr := testError{
		err:   errors.New("test error"),
		label: "test error",
	}
	tests := []struct {
		description   string
		msg           string
		underlyingErr error
		expectedLabel string
	}{
		{
			description:   "No underlying error",
			expectedLabel: invalidEventLabel,
		},
		{
			description:   "Underlying error",
			underlyingErr: errors.New("test error"),
			expectedLabel: invalidEventLabel,
		},
		{
			description:   "Underlying error label",
			underlyingErr: testErr,
			expectedLabel: "test_error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			err := InvalidEventErr{OriginalErr: tc.underlyingErr}
			if tc.underlyingErr != nil {
				assert.Contains(err.Error(), tc.underlyingErr.Error())
			}
			assert.Equal(tc.underlyingErr, err.Unwrap())
			assert.Equal(tc.expectedLabel, err.ErrorLabel())
		})
	}
}

func TestInvalidBootTimeErr(t *testing.T) {
	tests := []struct {
		description   string
		underlyingErr error
		expectedLabel string
	}{
		{
			description:   "No underlying error",
			expectedLabel: invalidBootTimeLabel,
		},
		{
			description:   "Underlying error",
			underlyingErr: errors.New("test error"),
			expectedLabel: invalidBootTimeLabel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			err := InvalidBootTimeErr{OriginalErr: tc.underlyingErr}
			if tc.underlyingErr != nil {
				assert.Contains(err.Error(), tc.underlyingErr.Error())
			}
			assert.Equal(tc.underlyingErr, err.Unwrap())
			assert.Equal(tc.expectedLabel, err.ErrorLabel())
		})
	}
}

func TestInvalidBirthdateErr(t *testing.T) {
	tests := []struct {
		description   string
		underlyingErr error
		expectedLabel string
	}{
		{
			description:   "No underlying error",
			expectedLabel: invalidBirthdateLabel,
		},
		{
			description:   "Underlying error",
			underlyingErr: errors.New("test error"),
			expectedLabel: invalidBirthdateLabel,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)
			err := InvalidBirthdateErr{OriginalErr: tc.underlyingErr}
			if tc.underlyingErr != nil {
				assert.Contains(err.Error(), tc.underlyingErr.Error())
			}
			assert.Equal(tc.underlyingErr, err.Unwrap())
			assert.Equal(tc.expectedLabel, err.ErrorLabel())
		})
	}
}
