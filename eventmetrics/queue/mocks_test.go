package queue

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/glaukos/message"
)

type mockParser struct {
	mock.Mock
}

func (mp *mockParser) Parse(event message.Event) error {
	args := mp.Called(event)
	return args.Error(0)
}
