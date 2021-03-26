package events

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
)

type mockValidator struct {
	mock.Mock
}

func (m *mockValidator) Valid(e interpreter.Event) (bool, error) {
	args := m.Called(e)
	return args.Bool(0), args.Error(1)
}
