package parsing

import "github.com/stretchr/testify/mock"

type mockCodexClient struct {
	mock.Mock
}

func (m *mockCodexClient) GetEvents(device string) []Event {
	args := m.Called(device)
	return args.Get(0).([]Event)
}
