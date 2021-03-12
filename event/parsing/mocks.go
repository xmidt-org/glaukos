package parsing

import (
	"time"

	"github.com/stretchr/testify/mock"
)

type mockTimeValidation struct {
	mock.Mock
}

func (m *mockTimeValidation) IsTimeValid(t time.Time) (bool, error) {
	args := m.Called(t)
	return args.Bool(0), args.Error(1)
}

func (m *mockTimeValidation) CurrentTime() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}
