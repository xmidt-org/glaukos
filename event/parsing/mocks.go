package parsing

import (
	"time"

	"github.com/stretchr/testify/mock"
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
