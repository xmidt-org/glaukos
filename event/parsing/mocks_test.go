package parsing

import "github.com/stretchr/testify/mock"

type mockEventClient struct {
	mock.Mock
}

func (m *mockEventClient) GetEvents(device string) []Event {
	args := m.Called(device)
	return args.Get(0).([]Event)
}
