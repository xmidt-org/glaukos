package queue

import (
	"github.com/stretchr/testify/mock"
)

type MockParser struct {
	mock.Mock
}

func (mp *MockParser) Parse(wrpWithTime WrpWithTime) error {
	args := mp.Called(wrpWithTime)
	return args.Error(0)
}

func (mp *MockParser) Name() string {
	args := mp.Called()
	return args.String(0)
}
