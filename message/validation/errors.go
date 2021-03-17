package validation

import (
	"errors"
	"fmt"
	"strings"
)

const (
	invalidEventLabel     = "invalid_event_err"
	invalidBootTimeLabel  = "invalid_boot_time"
	invalidBirthdateLabel = "invalid_birthdate"
)

// MetricsLogError is an optional interface for errors to implement if the error should be
// logged by prometheus metrics with a certain label.
type MetricsLogError interface {
	ErrorLabel() string
}

type InvalidEventErr struct {
	OriginalErr error
}

func (e InvalidEventErr) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("event invalid: %v", e.OriginalErr)
	}
	return "event invalid"
}

func (e InvalidEventErr) Unwrap() error {
	return e.OriginalErr
}

func (e InvalidEventErr) ErrorLabel() string {
	var err MetricsLogError
	if ok := errors.As(e.OriginalErr, &err); ok {
		return strings.Replace(err.ErrorLabel(), " ", "_", -1)
	}

	return invalidEventLabel
}

type InvalidBootTimeErr struct {
	OriginalErr error
}

func (e InvalidBootTimeErr) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("boot-time invalid: %v", e.OriginalErr)
	}
	return "boot-time invalid"
}

func (e InvalidBootTimeErr) Unwrap() error {
	return e.OriginalErr
}

func (e InvalidBootTimeErr) ErrorLabel() string {
	return invalidBootTimeLabel
}

type InvalidBirthdateErr struct {
	OriginalErr error
}

func (e InvalidBirthdateErr) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("birthdate invalid: %v", e.OriginalErr)
	}
	return "birthdate invalid"
}

func (e InvalidBirthdateErr) Unwrap() error {
	return e.OriginalErr
}

func (e InvalidBirthdateErr) ErrorLabel() string {
	return invalidBirthdateLabel
}
