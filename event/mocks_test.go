package event

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/wrp-go/v3"
)

type mockParser struct {
	mock.Mock
}

func (mp *mockParser) Parse(msg wrp.Message) error {
	args := mp.Called(msg)
	return args.Error(0)
}
