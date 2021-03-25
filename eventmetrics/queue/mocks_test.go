package queue

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
)

type mockParser struct {
	mock.Mock
}

func (mp *mockParser) Parse(event interpreter.Event) error {
	args := mp.Called(event)
	return args.Error(0)
}
