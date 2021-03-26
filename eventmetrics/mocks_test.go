package eventmetrics

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
)

type mockQueue struct {
	mock.Mock
}

func (m *mockQueue) Queue(e interpreter.Event) error {
	args := m.Called(e)
	return args.Error(0)
}
