package queue

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
)

type mockParser struct {
	mock.Mock
}

func (mp *mockParser) Parse(event interpreter.Event) {
	mp.Called(event)
}

func (mp *mockParser) Name() string {
	args := mp.Called()
	return args.String(0)
}
