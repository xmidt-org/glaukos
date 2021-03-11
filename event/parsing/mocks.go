package parsing

import (
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/event/client"
	"github.com/xmidt-org/glaukos/event/queue"
)

type MockTimeValidation struct {
	mock.Mock
}

func (m *MockTimeValidation) IsTimeValid(t time.Time) (bool, error) {
	args := m.Called(t)
	return args.Bool(0), args.Error(1)
}

func (m *MockTimeValidation) CurrentTime() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

type MockEventValidation struct {
	mock.Mock
}

func (m *MockEventValidation) IsEventValid(e client.Event) (bool, error) {
	args := m.Called(e)
	return args.Bool(0), args.Error(1)
}

func (m *MockEventValidation) IsWRPValid(msg queue.WrpWithTime) (bool, error) {
	args := m.Called(msg)
	return args.Bool(0), args.Error(1)
}

func (m *MockEventValidation) GetEventCompareTime(e client.Event) (time.Time, error) {
	args := m.Called(e)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *MockEventValidation) GetWRPCompareTime(msg queue.WrpWithTime) (time.Time, error) {
	args := m.Called(msg)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *MockEventValidation) ValidateType(dest string) bool {
	args := m.Called(dest)
	return args.Bool(0)
}

func (m *MockEventValidation) DuplicateAllowed() bool {
	args := m.Called()
	return args.Bool(0)
}
