package validation

import (
	"fmt"
	"strings"
)

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
	if m, ok := (e.OriginalErr).(MetricsLogError); ok {
		return strings.Replace(m.ErrorLabel(), " ", "_", -1)
	}

	return "invalid_event_err"
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
	return "invalid_boot_time"
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
	return "invalid_birthdate"
}
