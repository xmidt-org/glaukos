package history

import (
	"github.com/stretchr/testify/mock"
)

type MockEventClient struct {
	mock.Mock
}

func (m *MockEventClient) GetEvents(device string) []Event {
	args := m.Called(device)
	return args.Get(0).([]Event)
}
