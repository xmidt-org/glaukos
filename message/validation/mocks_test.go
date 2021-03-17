package validation

import (
	"github.com/stretchr/testify/mock"
)

type mockError struct {
	mock.Mock
}

func (m *mockError) Error() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockError) ErrorLabel() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockError) Unwrap() error {
	args := m.Called()
	return args.Error(0)
}
