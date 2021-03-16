package parsers

import (
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/message"
)

type mockEventClient struct {
	mock.Mock
}

func (m *mockEventClient) GetEvents(device string) []message.Event {
	args := m.Called(device)
	return args.Get(0).([]message.Event)
}

type mockEventValidation struct {
	mock.Mock
}

func (m *mockEventValidation) ValidateEvent(e message.Event) (bool, error) {
	args := m.Called(e)
	return args.Bool(0), args.Error(1)
}

func (m *mockEventValidation) GetCompareTime(e message.Event) (time.Time, error) {
	args := m.Called(e)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *mockEventValidation) ValidateType(dest string) bool {
	args := m.Called(dest)
	return args.Bool(0)
}
