package history

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/message"
)

type mockValidator struct {
	mock.Mock
}

func (m *mockValidator) Valid(e message.Event) (bool, error) {
	args := m.Called(e)
	return args.Bool(0), args.Error(1)
}
