package parsers

import (
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/event/history"
	"github.com/xmidt-org/glaukos/event/queue"
)

type mockEventValidation struct {
	mock.Mock
}

func (m *mockEventValidation) IsEventValid(e history.Event) (bool, error) {
	args := m.Called(e)
	return args.Bool(0), args.Error(1)
}

func (m *mockEventValidation) IsWRPValid(msg queue.WrpWithTime) (bool, error) {
	args := m.Called(msg)
	return args.Bool(0), args.Error(1)
}

func (m *mockEventValidation) GetEventCompareTime(e history.Event) (time.Time, error) {
	args := m.Called(e)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *mockEventValidation) GetWRPCompareTime(msg queue.WrpWithTime) (time.Time, error) {
	args := m.Called(msg)
	return args.Get(0).(time.Time), args.Error(1)
}

func (m *mockEventValidation) ValidateType(dest string) bool {
	args := m.Called(dest)
	return args.Bool(0)
}
