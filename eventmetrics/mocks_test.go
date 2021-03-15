package eventmetrics

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/message"
)

type mockQueue struct {
	mock.Mock
}

func (m *mockQueue) Queue(e message.Event) error {
	args := m.Called(e)
	return args.Error(0)
}
