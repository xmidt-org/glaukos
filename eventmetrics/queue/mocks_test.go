package queue

import (
	"github.com/stretchr/testify/mock"
)

type mockParser struct {
	mock.Mock
}

func (mp *mockParser) Parse(wrpWithTime WrpWithTime) error {
	args := mp.Called(wrpWithTime)
	return args.Error(0)
}
