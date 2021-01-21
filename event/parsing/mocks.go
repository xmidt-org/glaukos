package parsing

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/wrp-go/v3"
)

type MockParser struct {
	mock.Mock
}

func (mp *MockParser) Parse(msg wrp.Message) error {
	args := mp.Called(msg)
	return args.Error(0)
}
