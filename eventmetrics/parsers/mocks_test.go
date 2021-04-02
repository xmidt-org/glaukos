package parsers

import (
	"github.com/stretchr/testify/mock"
	"github.com/xmidt-org/interpreter"
)

type mockValidator struct {
	mock.Mock
}

func (m *mockValidator) Valid(e interpreter.Event) (bool, error) {
	args := m.Called(e)
	return args.Bool(0), args.Error(1)
}

type mockEventClient struct {
	mock.Mock
}

func (m *mockEventClient) GetEvents(deviceID string) []interpreter.Event {
	args := m.Called(deviceID)
	return args.Get(0).([]interpreter.Event)
}

type mockFinder struct {
	mock.Mock
}

func (m *mockFinder) Find(events []interpreter.Event, incomingEvent interpreter.Event) (interpreter.Event, error) {
	args := m.Called(events, incomingEvent)
	return args.Get(0).(interpreter.Event), args.Error(1)
}

type testErrorWithEvent struct {
	err   error
	event interpreter.Event
}

func (t testErrorWithEvent) Error() string {
	return t.err.Error()
}

func (t testErrorWithEvent) Event() interpreter.Event {
	return t.event
}
